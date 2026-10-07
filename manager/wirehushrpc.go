/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */
package manager

import (
	"context"
	"errors"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/windows/bootstrap"
	"golang.zx2c4.com/wireguard/windows/conf"
	"golang.zx2c4.com/wireguard/windows/protocol"
	"golang.zx2c4.com/wireguard/windows/version"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"
)

type wireHushSessionKey struct{}
type wireHushRPCSession struct {
	identity       wireHushPipeIdentity
	ready, closing bool
}
type wireHushRPCServer struct {
	protocol.UnimplementedManagerServer
	mutations     *wireHushMutations
	instance      string
	revision      atomic.Uint64
	sessionsLock  sync.Mutex
	sessions      map[*wireHushRPCSession]bool
	stopping      bool // protected by sessionsLock
	stopRequested chan struct{}
	shutdown      chan struct{}
	readBootstrap func() (bootstrap.Settings, error)
	saveBootstrap func(bootstrap.Settings) error
}

func newWireHushRPCServer() (*wireHushRPCServer, error) {
	id, err := conf.NewTunnelID()
	if err != nil {
		return nil, err
	}
	legacy := &ManagerService{}
	return &wireHushRPCServer{mutations: newWireHushMutations(), instance: id.String(), sessions: make(map[*wireHushRPCSession]bool), stopRequested: make(chan struct{}, 1), shutdown: make(chan struct{}), readBootstrap: legacy.BootstrapSettings, saveBootstrap: legacy.SaveBootstrapSettings}, nil
}

func (server *wireHushRPCServer) TagConn(ctx context.Context, _ *stats.ConnTagInfo) context.Context {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return ctx
	}
	identity, ok := p.AuthInfo.(wireHushPipeIdentity)
	if !ok || validateWireHushPipeIdentity(identity) != nil {
		return ctx
	}
	session := &wireHushRPCSession{identity: identity}
	server.sessionsLock.Lock()
	if server.stopping {
		server.sessionsLock.Unlock()
		return ctx
	}
	server.sessions[session] = true
	server.sessionsLock.Unlock()
	return context.WithValue(ctx, wireHushSessionKey{}, session)
}
func (server *wireHushRPCServer) HandleConn(ctx context.Context, event stats.ConnStats) {
	if _, ok := event.(*stats.ConnEnd); !ok {
		return
	}
	session, _ := ctx.Value(wireHushSessionKey{}).(*wireHushRPCSession)
	server.sessionsLock.Lock()
	delete(server.sessions, session)
	server.sessionsLock.Unlock()
}
func (*wireHushRPCServer) TagRPC(ctx context.Context, _ *stats.RPCTagInfo) context.Context {
	return ctx
}
func (*wireHushRPCServer) HandleRPC(context.Context, stats.RPCStats) {}

func (server *wireHushRPCServer) session(ctx context.Context, requireReady bool) (*wireHushRPCSession, error) {
	session, ok := ctx.Value(wireHushSessionKey{}).(*wireHushRPCSession)
	if !ok || session == nil || validateWireHushPipeIdentity(session.identity) != nil {
		return nil, status.Error(codes.Unauthenticated, "WireHush authentication is required")
	}
	server.sessionsLock.Lock()
	defer server.sessionsLock.Unlock()
	if !server.sessions[session] || session.closing {
		return nil, status.Error(codes.Unavailable, "WireHush session is closed")
	}
	if requireReady && !session.ready {
		return nil, status.Error(codes.FailedPrecondition, "WireHush version handshake is required")
	}
	return session, nil
}

func (server *wireHushRPCServer) unary(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	ctx, cancel := context.WithTimeout(ctx, protocol.RequestTimeout)
	defer cancel()
	if _, err := server.session(ctx, info.FullMethod != protocol.Manager_Handshake_FullMethodName); err != nil {
		return nil, err
	}
	reply, err := handler(ctx, request)
	return reply, wireHushRPCError(err)
}

func wireHushRPCError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := status.FromError(err); ok {
		return err
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "WireHush operation timed out; refresh state before retrying")
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "WireHush operation was canceled; refresh state")
	case errors.Is(err, errWireHushAccessDenied), errors.Is(err, windows.ERROR_ACCESS_DENIED):
		return status.Error(codes.PermissionDenied, "This account is not allowed to perform that operation")
	case errors.Is(err, os.ErrNotExist), errors.Is(err, windows.ERROR_FILE_NOT_FOUND):
		return status.Error(codes.NotFound, "Tunnel is unavailable")
	case errors.Is(err, conf.ErrTunnelNameInUse):
		return status.Error(codes.AlreadyExists, "A tunnel with that name already exists in this access scope")
	case errors.Is(err, errWireHushDeviceBusy):
		return status.Error(codes.FailedPrecondition, "WireHush is currently active; stop the active tunnel first")
	case errors.Is(err, errWireHushTunnelActive):
		return status.Error(codes.FailedPrecondition, "Stop this tunnel before editing it")
	case errors.Is(err, errWireHushClosing):
		return status.Error(codes.Unavailable, "WireHush is closing")
	case errors.Is(err, errWireHushTunnelServiceOwnershipConflict):
		return status.Error(codes.FailedPrecondition, "A service ownership conflict requires administrator repair")
	default:
		return status.Error(codes.Internal, "WireHush could not complete the operation; protected state was retained")
	}
}

func wireHushRPCReference(caller wireHushCaller, reference *protocol.TunnelRef) (conf.TunnelServiceLocator, error) {
	if reference == nil {
		return conf.TunnelServiceLocator{}, status.Error(codes.InvalidArgument, "Select a tunnel")
	}
	id, err := conf.ParseTunnelID(reference.TunnelId)
	if err != nil {
		return conf.TunnelServiceLocator{}, status.Error(codes.InvalidArgument, "Invalid tunnel identity")
	}
	locator := conf.TunnelServiceLocator{TunnelID: id}
	switch reference.Scope {
	case protocol.Scope_SCOPE_PRIVATE:
		locator.Scope = conf.TunnelScopePrivate
		locator.OwnerSID = caller.SID
	case protocol.Scope_SCOPE_SHARED:
		locator.Scope = conf.TunnelScopeShared
	default:
		return conf.TunnelServiceLocator{}, status.Error(codes.InvalidArgument, "Invalid tunnel access scope")
	}
	return locator, nil
}
func wireHushProtocolReference(locator conf.TunnelServiceLocator) *protocol.TunnelRef {
	return &protocol.TunnelRef{TunnelId: locator.TunnelID.String(), Scope: protocol.Scope(locator.Scope)}
}
func wireHushProtocolState(state TunnelState) protocol.TunnelState {
	switch state {
	case TunnelStopped:
		return protocol.TunnelState_TUNNEL_STATE_STOPPED
	case TunnelStarting:
		return protocol.TunnelState_TUNNEL_STATE_STARTING
	case TunnelStarted:
		return protocol.TunnelState_TUNNEL_STATE_CONNECTED
	case TunnelStopping:
		return protocol.TunnelState_TUNNEL_STATE_STOPPING
	default:
		return protocol.TunnelState_TUNNEL_STATE_UNKNOWN
	}
}

func (server *wireHushRPCServer) Handshake(ctx context.Context, request *protocol.HandshakeRequest) (*protocol.HandshakeReply, error) {
	session, err := server.session(ctx, false)
	if err != nil {
		return nil, err
	}
	if err := protocol.ValidateVersion(request); err != nil {
		return nil, err
	}
	server.sessionsLock.Lock()
	session.ready = true
	server.sessionsLock.Unlock()
	admin := session.identity.Caller.Administrator
	capabilities := []protocol.Capability{protocol.Capability_CAPABILITY_TUNNELS, protocol.Capability_CAPABILITY_ENCRYPTED_DNS, protocol.Capability_CAPABILITY_BOOTSTRAP_SETTINGS, protocol.Capability_CAPABILITY_EVENTS, protocol.Capability_CAPABILITY_EXPORT, protocol.Capability_CAPABILITY_SESSION_EXIT}
	if admin {
		capabilities = append(capabilities, protocol.Capability_CAPABILITY_SHARED_CREATE)
	}
	return &protocol.HandshakeReply{ProtocolMajor: protocol.Major, ProtocolMinor: protocol.Minor, ProductVersion: version.Number, InstanceId: server.instance, Capabilities: capabilities, MayEditShared: admin, MayEditMachineSettings: admin}, nil
}

func (server *wireHushRPCServer) Snapshot(ctx context.Context, _ *protocol.Empty) (*protocol.SnapshotReply, error) {
	session, err := server.session(ctx, true)
	if err != nil {
		return nil, err
	}
	select {
	case server.mutations.gate <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer server.mutations.leave()
	caller := session.identity.Caller
	reply := &protocol.SnapshotReply{InstanceId: server.instance, ManagerClosing: server.mutations.closing}
	for _, scope := range []conf.TunnelScope{conf.TunnelScopePrivate, conf.TunnelScopeShared} {
		owner := ""
		if scope == conf.TunnelScopePrivate {
			owner = caller.SID
		}
		metadata, err := server.mutations.control.ListTunnelMetadata(caller, scope, owner)
		if err != nil {
			return nil, err
		}
		for _, entry := range metadata {
			locator := conf.TunnelServiceLocator{TunnelID: entry.TunnelID, Scope: scope, OwnerSID: owner}
			state, err := server.mutations.control.state(locator)
			if err != nil {
				return nil, err
			}
			record, err := server.mutations.control.loadRecord(scope, owner, entry.TunnelID)
			if err != nil {
				return nil, err
			}
			config, err := wireHushStoredConfigFromRecord(record)
			if err != nil {
				return nil, err
			}
			tunnel := &protocol.TunnelSnapshot{Tunnel: wireHushProtocolReference(locator), Name: entry.Name, State: wireHushProtocolState(state), EncryptedDns: len(config.Interface.DNSOverHTTPS) > 0, MayEdit: scope == conf.TunnelScopePrivate || caller.Administrator, MayExport: scope == conf.TunnelScopePrivate || caller.Administrator}
			if tunnel.MayExport {
				tunnel.Network = wireHushNetworkDetails(config, locator)
			}
			if state == TunnelStarted {
				runtime, err := server.mutations.control.runtimeConfig(locator)
				if err == nil {
					var rx, tx uint64
					var last int64
					for _, p := range runtime.Peers {
						rx += uint64(p.RxBytes)
						tx += uint64(p.TxBytes)
						if stamp := int64(p.LastHandshakeTime) / int64(time.Second); stamp > last {
							last = stamp
						}
						if tunnel.Network != nil {
							for _, visible := range tunnel.Network.Peers {
								if visible.PublicKey != p.PublicKey.String() {
									continue
								}
								peerRx, peerTx := uint64(p.RxBytes), uint64(p.TxBytes)
								visible.RxBytes, visible.TxBytes = &peerRx, &peerTx
								stamp := int64(p.LastHandshakeTime) / int64(time.Second)
								if stamp != 0 {
									visible.LatestHandshakeUnix = &stamp
								}
							}
						}
					}
					tunnel.RxBytes = &rx
					tunnel.TxBytes = &tx
					if last != 0 {
						tunnel.LatestHandshakeUnix = &last
					}
				} else {
					tunnel.State = protocol.TunnelState_TUNNEL_STATE_UNKNOWN
				}
			}
			reply.Tunnels = append(reply.Tunnels, tunnel)
		}
	}
	active, err := server.mutations.inventory()
	if err != nil {
		return nil, err
	}
	for _, tunnel := range active {
		if tunnel.State != TunnelStopped && authorizeWireHushTunnel(caller, tunnel.Locator, wireHushTunnelMetadataRead) != nil {
			reply.DeviceBusyForAnotherUser = true
		}
	}
	reply.Revision = server.revision.Add(1)
	return reply, nil
}

func wireHushNetworkDetails(config *conf.Config, locator conf.TunnelServiceLocator) *protocol.TunnelNetwork {
	name, _ := conf.WireHushAdapterNameOfTunnelID(locator.TunnelID)
	network := &protocol.TunnelNetwork{InterfaceName: name}
	for _, address := range config.Interface.Addresses {
		if address.Addr().Is4() {
			network.Ipv4Addresses = append(network.Ipv4Addresses, address.String())
		} else {
			network.Ipv6Addresses = append(network.Ipv6Addresses, address.String())
		}
	}
	if config.Interface.ListenPort != 0 {
		port := uint32(config.Interface.ListenPort)
		network.ListenPort = &port
	}
	for _, address := range config.Interface.DNS {
		network.DnsServers = append(network.DnsServers, address.String())
	}
	for _, raw := range config.Interface.DNSOverHTTPS {
		if resolver, err := url.Parse(raw); err == nil {
			resolver.User = nil
			resolver.RawQuery = ""
			resolver.Fragment = ""
			network.DnsServers = append(network.DnsServers, resolver.String())
		}
	}
	for _, p := range config.Peers {
		peer := &protocol.TunnelPeer{PublicKey: p.PublicKey.String()}
		if !p.Endpoint.IsEmpty() {
			peer.EndpointDisplay = p.Endpoint.String()
			if network.EndpointDisplay == "" {
				network.EndpointDisplay = peer.EndpointDisplay
			}
		}
		for _, route := range p.AllowedIPs {
			peer.AllowedIps = append(peer.AllowedIps, route.String())
			network.AllowedIps = append(network.AllowedIps, route.String())
		}
		if p.PersistentKeepalive != 0 {
			seconds := uint32(p.PersistentKeepalive)
			peer.KeepaliveSeconds = &seconds
		}
		network.Peers = append(network.Peers, peer)
	}
	return network
}

func (server *wireHushRPCServer) ReadConfiguration(ctx context.Context, reference *protocol.TunnelRef) (*protocol.ConfigurationReply, error) {
	session, err := server.session(ctx, true)
	if err != nil {
		return nil, err
	}
	locator, err := wireHushRPCReference(session.identity.Caller, reference)
	if err != nil {
		return nil, err
	}
	config, err := server.mutations.control.StoredTunnelConfig(session.identity.Caller, locator)
	if err != nil {
		return nil, err
	}
	return &protocol.ConfigurationReply{WgQuickText: config.ToWgQuick()}, nil
}
func validateWireHushRPCConfiguration(name, text string) error {
	if !conf.TunnelNameIsValid(name) || len(text) > protocol.MaximumMessageBytes {
		return status.Error(codes.InvalidArgument, "Enter a valid tunnel name and configuration")
	}
	if _, err := conf.FromWgQuick(text, name); err != nil {
		return status.Error(codes.InvalidArgument, "Tunnel configuration is invalid")
	}
	return nil
}
func (server *wireHushRPCServer) CreateTunnel(ctx context.Context, request *protocol.CreateTunnelRequest) (*protocol.TunnelSnapshot, error) {
	session, err := server.session(ctx, true)
	if err != nil {
		return nil, err
	}
	if err := validateWireHushRPCConfiguration(request.Name, request.WgQuickText); err != nil {
		return nil, err
	}
	if request.Scope != protocol.Scope_SCOPE_PRIVATE && request.Scope != protocol.Scope_SCOPE_SHARED {
		return nil, status.Error(codes.InvalidArgument, "Invalid tunnel access scope")
	}
	metadata, err := server.mutations.Create(ctx, session.identity.Caller, conf.TunnelScope(request.Scope), request.Name, request.WgQuickText)
	if err != nil {
		return nil, err
	}
	return &protocol.TunnelSnapshot{Tunnel: wireHushProtocolReference(conf.TunnelServiceLocator{TunnelID: metadata.TunnelID, Scope: metadata.Scope, OwnerSID: metadata.OwnerSID}), Name: metadata.Name, State: protocol.TunnelState_TUNNEL_STATE_STOPPED, MayEdit: true, MayExport: true}, nil
}
func (server *wireHushRPCServer) UpdateTunnel(ctx context.Context, request *protocol.UpdateTunnelRequest) (*protocol.Empty, error) {
	session, err := server.session(ctx, true)
	if err != nil {
		return nil, err
	}
	locator, err := wireHushRPCReference(session.identity.Caller, request.Tunnel)
	if err != nil {
		return nil, err
	}
	if err := validateWireHushRPCConfiguration(request.Name, request.WgQuickText); err != nil {
		return nil, err
	}
	return &protocol.Empty{}, server.mutations.Update(ctx, session.identity.Caller, locator, request.Name, request.WgQuickText)
}
func (server *wireHushRPCServer) tunnelAction(ctx context.Context, reference *protocol.TunnelRef, action func(context.Context, wireHushCaller, conf.TunnelServiceLocator) error) (*protocol.Empty, error) {
	session, err := server.session(ctx, true)
	if err != nil {
		return nil, err
	}
	locator, err := wireHushRPCReference(session.identity.Caller, reference)
	if err != nil {
		return nil, err
	}
	return &protocol.Empty{}, action(ctx, session.identity.Caller, locator)
}
func (server *wireHushRPCServer) StartTunnel(ctx context.Context, reference *protocol.TunnelRef) (*protocol.Empty, error) {
	return server.tunnelAction(ctx, reference, server.mutations.Start)
}
func (server *wireHushRPCServer) StopTunnel(ctx context.Context, reference *protocol.TunnelRef) (*protocol.Empty, error) {
	return server.tunnelAction(ctx, reference, server.mutations.Stop)
}
func (server *wireHushRPCServer) DeleteTunnel(ctx context.Context, reference *protocol.TunnelRef) (*protocol.Empty, error) {
	return server.tunnelAction(ctx, reference, server.mutations.Delete)
}
func (server *wireHushRPCServer) ReadBootstrap(ctx context.Context, _ *protocol.Empty) (*protocol.BootstrapSettings, error) {
	if _, err := server.session(ctx, true); err != nil {
		return nil, err
	}
	settings, err := server.readBootstrap()
	if err != nil {
		return nil, err
	}
	reply := &protocol.BootstrapSettings{}
	for _, entry := range settings.Entries {
		reply.Resolvers = append(reply.Resolvers, &protocol.BootstrapResolver{Address: entry.Address.String(), Enabled: entry.Enabled, Custom: entry.Custom})
	}
	return reply, nil
}
func (server *wireHushRPCServer) SaveBootstrap(ctx context.Context, request *protocol.BootstrapSettings) (*protocol.Empty, error) {
	session, err := server.session(ctx, true)
	if err != nil {
		return nil, err
	}
	if !session.identity.Caller.Administrator {
		return nil, errWireHushAccessDenied
	}
	if len(request.Resolvers) > 64 {
		return nil, status.Error(codes.InvalidArgument, "Too many bootstrap resolvers")
	}
	settings := bootstrap.Settings{}
	for _, entry := range request.Resolvers {
		address, err := bootstrap.ParseAddress(entry.Address)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "Enter valid bootstrap resolver IP addresses")
		}
		settings.Entries = append(settings.Entries, bootstrap.Entry{Address: address, Enabled: entry.Enabled, Custom: entry.Custom})
	}
	if err := settings.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, "Enable at least one unique bootstrap resolver")
	}
	if err := server.mutations.enter(ctx); err != nil {
		return nil, err
	}
	defer server.mutations.leave()
	return &protocol.Empty{}, server.saveBootstrap(settings)
}

func (server *wireHushRPCServer) Subscribe(_ *protocol.Empty, stream grpc.ServerStreamingServer[protocol.SnapshotReply]) error {
	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()
	if _, err := server.session(ctx, true); err != nil {
		return err
	}
	type update struct {
		snapshot *protocol.SnapshotReply
		err      error
	}
	latest := make(chan update, 1)
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			readCtx, readCancel := context.WithTimeout(ctx, protocol.RequestTimeout)
			snapshot, err := server.Snapshot(readCtx, &protocol.Empty{})
			readCancel()
			value := update{snapshot, err}
			select {
			case latest <- value:
			default:
				select {
				case <-latest:
				default:
				}
				select {
				case latest <- value:
				case <-ctx.Done():
					return
				}
			}
			if err != nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-server.shutdown:
				return
			case <-ticker.C:
			}
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-server.shutdown:
			return nil
		case value := <-latest:
			if value.err != nil {
				return wireHushRPCError(value.err)
			}
			if err := stream.Send(value.snapshot); err != nil {
				return err
			}
		}
	}
}

func (server *wireHushRPCServer) ExitSession(ctx context.Context, _ *protocol.Empty) (*protocol.ExitReply, error) {
	session, err := server.session(ctx, true)
	if err != nil {
		return nil, err
	}
	if err := server.mutations.enter(ctx); err != nil {
		return nil, err
	}
	defer server.mutations.leave()
	server.sessionsLock.Lock()
	others := len(server.sessions) > 1
	if others {
		session.closing = true
		delete(server.sessions, session)
	}
	server.sessionsLock.Unlock()
	if others {
		return &protocol.ExitReply{CleanupComplete: true}, nil
	}
	active, err := server.mutations.inventory()
	if err != nil {
		return nil, err
	}
	for _, tunnel := range active {
		if tunnel.State == TunnelStopped {
			continue
		}
		if authorizeWireHushTunnel(session.identity.Caller, tunnel.Locator, wireHushTunnelControl) != nil {
			continue
		}
		if err := server.mutations.control.withContext(ctx).StopTunnel(session.identity.Caller, tunnel.Locator); err != nil {
			return nil, err
		}
	}
	active, err = server.mutations.inventory()
	if err != nil {
		return nil, err
	}
	stop := true
	for _, tunnel := range active {
		if tunnel.State != TunnelStopped {
			stop = false
		}
	}
	server.sessionsLock.Lock()
	// A new authenticated connection arriving during cleanup must keep the
	// manager alive; it is not closed merely because this UI began exiting.
	if len(server.sessions) > 1 {
		stop = false
	}
	if stop {
		server.stopping = true
	}
	session.closing = true
	delete(server.sessions, session)
	server.sessionsLock.Unlock()
	if stop {
		server.mutations.closing = true
		select {
		case server.stopRequested <- struct{}{}:
		default:
		}
	}
	return &protocol.ExitReply{CleanupComplete: true, ManagerStopping: stop}, nil
}
