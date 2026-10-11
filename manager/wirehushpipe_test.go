/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package manager

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/windows/conf"
	"golang.zx2c4.com/wireguard/windows/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/peer"
)

func TestWireHushPipeAuthorizationAndDescriptor(t *testing.T) {
	for _, caller := range []wireHushCaller{{}, {SID: "S-1-5-7", WireHushUser: true}, {SID: "S-1-5-2"}} {
		if validateWireHushPipeIdentity(wireHushPipeIdentity{Caller: caller}) == nil {
			t.Fatalf("unauthorized identity accepted: %+v", caller)
		}
	}
	for _, caller := range []wireHushCaller{{SID: wireHushAuthOtherSID}, {SID: wireHushAuthOwnerSID, Administrator: true}, {SID: wireHushAuthOwnerSID, WireHushUser: true}} {
		if err := validateWireHushPipeIdentity(wireHushPipeIdentity{Caller: caller}); err != nil {
			t.Fatal(err)
		}
	}
	group, err := windows.StringToSid("S-1-5-21-1-2-3-1001")
	if err != nil {
		t.Fatal(err)
	}
	sddl, err := wireHushPipeSecurityDescriptor(group)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := windows.SecurityDescriptorFromString(sddl); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{";;;WD)", ";;;BU)", "0x12019f"} {
		if strings.Contains(sddl, forbidden) {
			t.Fatalf("overbroad pipe descriptor: %s", sddl)
		}
	}
	if wireHushPipeClientAccess&0x4 != 0 {
		t.Fatal("client may create a server instance")
	}
	if _, err := wireHushPipeSecurityDescriptor(nil); err == nil {
		t.Fatal("missing group widened pipe descriptor")
	}
}

func TestWireHushGRPCSessionUsesAuthenticatedCaller(t *testing.T) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf(`\\.\pipe\WireHush.RPC.Test.%d.%d`, windows.GetCurrentProcessId(), time.Now().UnixNano())
	listener, err := winio.ListenPipe(path, &winio.PipeConfig{SecurityDescriptor: "O:" + user.User.Sid.String() + "G:" + user.User.Sid.String() + "D:P(A;;GA;;;" + user.User.Sid.String() + ")"})
	if err != nil {
		t.Fatal(err)
	}
	group, err := windows.CreateWellKnownSid(windows.WinBuiltinUsersSid)
	if err != nil {
		listener.Close()
		t.Fatal(err)
	}
	service, _, _ := testWireHushRPC(t)
	service.sessions = make(map[*wireHushRPCSession]bool)
	server := grpc.NewServer(grpc.Creds(wireHushPipeCredentials{group: group}), grpc.StatsHandler(service), grpc.UnaryInterceptor(service.unary))
	protocol.RegisterManagerServer(server, service)
	done := make(chan struct{})
	go func() { defer close(done); server.Serve(boundedWireHushListener(listener, wireHushMaximumConnections)) }()
	defer func() { server.Stop(); listener.Close(); <-done }()
	client, err := grpc.NewClient("passthrough:///wirehush-rpc-test", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
		return winio.DialPipeAccessImpLevel(ctx, path, wireHushPipeClientAccess, winio.PipeImpLevelIdentification)
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rpc := protocol.NewManagerClient(client)
	if _, err := rpc.Handshake(ctx, &protocol.HandshakeRequest{ProtocolMajor: protocol.Major}); err != nil {
		t.Fatal(err)
	}
	fixture := wireHushControlRecord(t, "12345678-1234-4abc-8def-1234567890ab", conf.TunnelScopePrivate, wireHushAuthOwnerSID, "Fixture")
	created, err := rpc.CreateTunnel(ctx, &protocol.CreateTunnelRequest{Name: "Authenticated", Scope: protocol.Scope_SCOPE_PRIVATE, WgQuickText: fixture.WGQuickText})
	if err != nil {
		t.Fatal(err)
	}
	id, err := conf.ParseTunnelID(created.Tunnel.TunnelId)
	if err != nil {
		t.Fatal(err)
	}
	record, err := service.mutations.control.loadRecord(conf.TunnelScopePrivate, user.User.Sid.String(), id)
	if err != nil || record.OwnerSID != user.User.Sid.String() {
		t.Fatalf("authenticated creation identity was not derived from the Windows token: %v", err)
	}
	if _, err := rpc.Snapshot(ctx, &protocol.Empty{}); err != nil {
		t.Fatal(err)
	}
}

type wireHushTransportTestServer struct {
	protocol.UnimplementedManagerServer
	calls      atomic.Int32
	identities chan wireHushPipeIdentity
}

func (server *wireHushTransportTestServer) Handshake(ctx context.Context, request *protocol.HandshakeRequest) (*protocol.HandshakeReply, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, errWireHushAccessDenied
	}
	identity, ok := p.AuthInfo.(wireHushPipeIdentity)
	if !ok || validateWireHushPipeIdentity(identity) != nil {
		return nil, errWireHushAccessDenied
	}
	server.calls.Add(1)
	if server.identities != nil {
		server.identities <- identity
	}
	if err := protocol.ValidateVersion(request); err != nil {
		return nil, err
	}
	return &protocol.HandshakeReply{ProtocolMajor: protocol.Major}, nil
}

func TestWireHushGRPCNamedPipeAuthenticatesWindowsToken(t *testing.T) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	// Isolated test endpoint and current-user descriptor permit an unelevated
	// test server. Production always resolves WireHush Users and uses SYSTEM ACL.
	path := fmt.Sprintf(`\\.\pipe\WireHush.Test.%d.%d`, windows.GetCurrentProcessId(), time.Now().UnixNano())
	sddl := "O:" + user.User.Sid.String() + "G:" + user.User.Sid.String() + "D:P(A;;GA;;;" + user.User.Sid.String() + ")"
	listener, err := winio.ListenPipe(path, &winio.PipeConfig{SecurityDescriptor: sddl})
	if err != nil {
		t.Fatal(err)
	}
	group, err := windows.CreateWellKnownSid(windows.WinBuiltinUsersSid)
	if err != nil {
		listener.Close()
		t.Fatal(err)
	}
	server := grpc.NewServer(grpc.Creds(wireHushPipeCredentials{group: group}), grpc.MaxRecvMsgSize(protocol.MaximumMessageBytes))
	service := &wireHushTransportTestServer{identities: make(chan wireHushPipeIdentity, 2)}
	protocol.RegisterManagerServer(server, service)
	done := make(chan struct{})
	go func() { defer close(done); server.Serve(boundedWireHushListener(listener, wireHushMaximumConnections)) }()
	defer func() {
		server.Stop()
		listener.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("pipe server did not stop")
		}
	}()
	connect := func(level winio.PipeImpLevel) *grpc.ClientConn {
		client, err := grpc.NewClient("passthrough:///wirehush-test", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return winio.DialPipeAccessImpLevel(ctx, path, wireHushPipeClientAccess, level)
		}))
		if err != nil {
			t.Fatal(err)
		}
		return client
	}
	client := connect(winio.PipeImpLevelIdentification)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	reply, err := protocol.NewManagerClient(client).Handshake(ctx, &protocol.HandshakeRequest{ProtocolMajor: protocol.Major})
	cancel()
	client.Close()
	if err != nil || reply.ProtocolMajor != protocol.Major || service.calls.Load() != 1 {
		t.Fatalf("authenticated gRPC result=%v error=%v calls=%d", reply, err, service.calls.Load())
	}
	if identity := <-service.identities; identity.Caller.SID != user.User.Sid.String() {
		t.Fatal("pipe authentication did not bind the actual Windows caller SID")
	}
	client = connect(winio.PipeImpLevelAnonymous)
	ctx, cancel = context.WithTimeout(context.Background(), 500*time.Millisecond)
	_, err = protocol.NewManagerClient(client).Handshake(ctx, &protocol.HandshakeRequest{ProtocolMajor: protocol.Major})
	cancel()
	client.Close()
	if err == nil || service.calls.Load() != 1 {
		t.Fatalf("anonymous pipe reached RPC: error=%v calls=%d", err, service.calls.Load())
	}
}
