/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */
package manager

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"golang.zx2c4.com/wireguard/windows/bootstrap"
	"golang.zx2c4.com/wireguard/windows/conf"
	"golang.zx2c4.com/wireguard/windows/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"
)

func testWireHushRPC(t *testing.T, records ...conf.TunnelRecord) (*wireHushRPCServer, context.Context, *wireHushRPCSession) {
	t.Helper()
	server, err := newWireHushRPCServer()
	if err != nil {
		t.Fatal(err)
	}
	server.mutations = testWireHushMutations(records...)
	server.readBootstrap = func() (bootstrap.Settings, error) { return bootstrap.DefaultSettings(), nil }
	server.saveBootstrap = func(bootstrap.Settings) error { return nil }
	session := &wireHushRPCSession{identity: wireHushPipeIdentity{Caller: wireHushCaller{SID: wireHushAuthOwnerSID, WireHushUser: true}, SessionID: 1}}
	server.sessions[session] = true
	ctx := context.WithValue(context.Background(), wireHushSessionKey{}, session)
	return server, ctx, session
}

func TestWireHushRPCHandshakeAndIdentityBoundary(t *testing.T) {
	server, ctx, _ := testWireHushRPC(t)
	if _, err := server.Snapshot(ctx, &protocol.Empty{}); status.Code(err) != codes.FailedPrecondition {
		t.Fatal(err)
	}
	if _, err := server.Handshake(ctx, &protocol.HandshakeRequest{ProtocolMajor: 2}); status.Code(err) != codes.FailedPrecondition {
		t.Fatal(err)
	}
	if _, err := server.Handshake(ctx, &protocol.HandshakeRequest{ProtocolMajor: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := server.Snapshot(context.Background(), &protocol.Empty{}); status.Code(err) != codes.Unauthenticated {
		t.Fatal(err)
	}
	id := "12345678-1234-4abc-8def-1234567890ab"
	locator, err := wireHushRPCReference(wireHushCaller{SID: wireHushAuthOwnerSID}, &protocol.TunnelRef{TunnelId: id, Scope: protocol.Scope_SCOPE_PRIVATE})
	if err != nil || locator.OwnerSID != wireHushAuthOwnerSID {
		t.Fatalf("derived private identity=%+v %v", locator, err)
	}
}

func TestWireHushRPCSnapshotDoesNotLeakOtherUser(t *testing.T) {
	a := wireHushControlRecord(t, "12345678-1234-4abc-8def-1234567890ab", conf.TunnelScopePrivate, wireHushAuthOwnerSID, "Visible")
	b := wireHushControlRecord(t, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", conf.TunnelScopePrivate, wireHushAuthOtherSID, "SECRET-NAME")
	server, ctx, _ := testWireHushRPC(t, a, b)
	server.Handshake(ctx, &protocol.HandshakeRequest{ProtocolMajor: 1})
	server.mutations.inventory = func() ([]wireHushTrackedTunnel, error) {
		return []wireHushTrackedTunnel{{Locator: wireHushControlLocator(b), State: TunnelStarted}}, nil
	}
	reply, err := server.Snapshot(ctx, &protocol.Empty{})
	if err != nil || len(reply.Tunnels) != 1 || reply.Tunnels[0].Name != a.Name || !reply.DeviceBusyForAnotherUser {
		t.Fatalf("filtered snapshot=%v error=%v", reply, err)
	}
	if strings.Contains(reply.String(), b.Name) || strings.Contains(reply.String(), b.TunnelID.String()) || strings.Contains(reply.String(), b.OwnerSID) {
		t.Fatal("another user's private metadata leaked")
	}
	if _, err := server.ReadConfiguration(ctx, &protocol.TunnelRef{TunnelId: b.TunnelID.String(), Scope: protocol.Scope_SCOPE_PRIVATE}); err == nil {
		t.Fatal("another user's private ID resolved")
	}
}

func TestWireHushRPCSharedMemberCannotExportOrModify(t *testing.T) {
	record := wireHushControlRecord(t, "12345678-1234-4abc-8def-1234567890ab", conf.TunnelScopeShared, "", "Shared")
	server, ctx, _ := testWireHushRPC(t, record)
	server.Handshake(ctx, &protocol.HandshakeRequest{ProtocolMajor: 1})
	ref := wireHushProtocolReference(wireHushControlLocator(record))
	if _, err := server.ReadConfiguration(ctx, ref); !errors.Is(err, errWireHushAccessDenied) {
		t.Fatal(err)
	}
	if _, err := server.DeleteTunnel(ctx, ref); !errors.Is(err, errWireHushAccessDenied) {
		t.Fatal(err)
	}
	if _, err := server.SaveBootstrap(ctx, &protocol.BootstrapSettings{}); !errors.Is(err, errWireHushAccessDenied) {
		t.Fatal(err)
	}
	snapshot, err := server.Snapshot(ctx, &protocol.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Tunnels[0].MayEdit || snapshot.Tunnels[0].MayExport {
		t.Fatal("shared member received forbidden capabilities")
	}
	if snapshot.Tunnels[0].Network != nil {
		t.Fatal("shared member received configuration-derived details")
	}
}

func TestWireHushRPCNetworkDetailsAreAuthorizedAndKeyFree(t *testing.T) {
	record := wireHushControlRecord(t, "12345678-1234-4abc-8def-1234567890ab", conf.TunnelScopePrivate, wireHushAuthOwnerSID, "Private")
	server, ctx, _ := testWireHushRPC(t, record)
	server.Handshake(ctx, &protocol.HandshakeRequest{ProtocolMajor: 1})
	snapshot, err := server.Snapshot(ctx, &protocol.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	network := snapshot.Tunnels[0].Network
	if network == nil || len(network.Ipv4Addresses) != 1 || network.Ipv4Addresses[0] != "10.0.0.2/32" {
		t.Fatal("owner did not receive actual configured network details")
	}
	if strings.Contains(snapshot.String(), "PrivateKey") || strings.Contains(snapshot.String(), "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=") {
		t.Fatal("snapshot exported a private key")
	}
	config, err := wireHushStoredConfigFromRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	config.Interface.DNSOverHTTPS = []string{"https://user:secret@dns.example/dns-query?credential=private#secret"}
	network = wireHushNetworkDetails(config, wireHushControlLocator(record))
	if len(network.DnsServers) != 1 || network.DnsServers[0] != "https://dns.example/dns-query" {
		t.Fatal("DNS display exposed credentials or query parameters")
	}
}

func TestWireHushRPCFailureMessagesRedactSecrets(t *testing.T) {
	secret := "privatekey=SENSITIVE doh=https://secret.example/path owner=S-1-5-21-123"
	if err := wireHushRPCError(errors.New(secret)); status.Code(err) != codes.Internal || strings.Contains(err.Error(), "SENSITIVE") || strings.Contains(err.Error(), "secret.example") {
		t.Fatal(err)
	}
	if err := validateWireHushRPCConfiguration("valid", secret); status.Code(err) != codes.InvalidArgument || strings.Contains(err.Error(), "SENSITIVE") {
		t.Fatal(err)
	}
}

func TestWireHushExitKeepsUIOpenOnFailedCleanup(t *testing.T) {
	record := wireHushControlRecord(t, "12345678-1234-4abc-8def-1234567890ab", conf.TunnelScopePrivate, wireHushAuthOwnerSID, "Visible")
	server, ctx, session := testWireHushRPC(t, record)
	server.Handshake(ctx, &protocol.HandshakeRequest{ProtocolMajor: 1})
	server.mutations.inventory = func() ([]wireHushTrackedTunnel, error) {
		return []wireHushTrackedTunnel{{Locator: wireHushControlLocator(record), State: TunnelStarted}}, nil
	}
	server.mutations.control.waitForStop = func(conf.TunnelServiceLocator) error { return context.DeadlineExceeded }
	if reply, err := server.ExitSession(ctx, &protocol.Empty{}); !errors.Is(err, context.DeadlineExceeded) || reply != nil {
		t.Fatalf("failed exit=%v %v", reply, err)
	}
	if session.closing || server.mutations.closing {
		t.Fatal("failed cleanup closed session or manager")
	}
	select {
	case <-server.stopRequested:
		t.Fatal("manager exited before cleanup")
	default:
	}
}

func TestWireHushExitPreservesOtherSessionsAndOtherUserTunnel(t *testing.T) {
	record := wireHushControlRecord(t, "12345678-1234-4abc-8def-1234567890ab", conf.TunnelScopePrivate, wireHushAuthOtherSID, "SECRET")
	server, ctx, _ := testWireHushRPC(t, record)
	server.Handshake(ctx, &protocol.HandshakeRequest{ProtocolMajor: 1})
	server.mutations.inventory = func() ([]wireHushTrackedTunnel, error) {
		return []wireHushTrackedTunnel{{Locator: wireHushControlLocator(record), State: TunnelStarted}}, nil
	}
	server.mutations.control.stop = func(conf.TunnelServiceLocator) error { t.Fatal("exit stopped another user"); return nil }
	reply, err := server.ExitSession(ctx, &protocol.Empty{})
	if err != nil || !reply.CleanupComplete || reply.ManagerStopping {
		t.Fatalf("other-user exit=%v %v", reply, err)
	}
	server, ctx, _ = testWireHushRPC(t)
	server.Handshake(ctx, &protocol.HandshakeRequest{ProtocolMajor: 1})
	server.sessions[&wireHushRPCSession{identity: wireHushPipeIdentity{Caller: wireHushCaller{SID: wireHushAuthOtherSID, WireHushUser: true}, SessionID: 2}, ready: true}] = true
	reply, err = server.ExitSession(ctx, &protocol.Empty{})
	if err != nil || !reply.CleanupComplete || reply.ManagerStopping || len(server.sessions) != 1 {
		t.Fatalf("multi-session exit=%v %v", reply, err)
	}
}

func TestWireHushDisconnectDoesNotStopTunnelAndIdleExitIsRechecked(t *testing.T) {
	server, ctx, _ := testWireHushRPC(t)
	server.Handshake(ctx, &protocol.HandshakeRequest{ProtocolMajor: 1})
	server.mutations.control.stop = func(conf.TunnelServiceLocator) error { t.Fatal("disconnect stopped VPN"); return nil }
	server.HandleConn(ctx, &stats.ConnEnd{})
	if len(server.sessions) != 0 {
		t.Fatal("disconnected session retained")
	}
	server.mutations.inventory = func() ([]wireHushTrackedTunnel, error) { return []wireHushTrackedTunnel{{State: TunnelUnknown}}, nil }
	if server.commitIdleExit(context.Background()) {
		t.Fatal("unknown networking state allowed idle exit")
	}
	server.mutations.inventory = func() ([]wireHushTrackedTunnel, error) { return nil, nil }
	if !server.commitIdleExit(context.Background()) {
		t.Fatal("idle manager would not stop")
	}
}

type wireHushTestStream struct {
	grpc.ServerStream
	ctx  context.Context
	send func(*protocol.SnapshotReply) error
}

func (stream wireHushTestStream) Context() context.Context                 { return stream.ctx }
func (stream wireHushTestStream) Send(reply *protocol.SnapshotReply) error { return stream.send(reply) }

func TestWireHushSlowEventReaderDoesNotBlockMutations(t *testing.T) {
	server, ctx, _ := testWireHushRPC(t)
	server.Handshake(ctx, &protocol.HandshakeRequest{ProtocolMajor: 1})
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	sending := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- server.Subscribe(&protocol.Empty{}, wireHushTestStream{ctx: ctx, send: func(*protocol.SnapshotReply) error { close(sending); <-ctx.Done(); return ctx.Err() }})
	}()
	select {
	case <-sending:
	case <-time.After(time.Second):
		t.Fatal("event stream did not start")
	}
	mutationCtx, mutationCancel := context.WithTimeout(ctx, time.Second)
	defer mutationCancel()
	if _, err := server.CreateTunnel(mutationCtx, &protocol.CreateTunnelRequest{Scope: protocol.Scope_SCOPE_PRIVATE, Name: "New", WgQuickText: wireHushControlWGQuick}); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("event cancellation did not complete")
	}
}

func TestWireHushRPCLocalPrivateRoundTripWithoutGroup(t *testing.T) {
	server, ctx, session := testWireHushRPC(t)
	session.identity.Caller.WireHushUser = false
	handshake, err := server.Handshake(ctx, &protocol.HandshakeRequest{ProtocolMajor: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, cap := range handshake.Capabilities {
		if cap == protocol.Capability_CAPABILITY_SHARED_CREATE {
			t.Fatal("ordinary user received shared creation")
		}
	}
	text := wireHushControlWGQuick + "\n# Preserve saved details and comments\n"
	created, err := server.CreateTunnel(ctx, &protocol.CreateTunnelRequest{Name: "LocalPrivate", Scope: protocol.Scope_SCOPE_PRIVATE, WgQuickText: text})
	if err != nil {
		t.Fatal(err)
	}
	reply, err := server.ReadConfiguration(ctx, created.Tunnel)
	if err != nil || reply.WgQuickText != text {
		t.Fatal("stored configuration failed exact editor round trip")
	}
	if _, err = server.CreateTunnel(ctx, &protocol.CreateTunnelRequest{Name: "ForbiddenShared", Scope: protocol.Scope_SCOPE_SHARED, WgQuickText: text}); !errors.Is(err, errWireHushAccessDenied) {
		t.Fatal("ordinary user shared creation was not denied")
	}
}
