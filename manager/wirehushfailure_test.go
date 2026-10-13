package manager

import (
	"strings"
	"testing"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.zx2c4.com/wireguard/windows/conf"
	"golang.zx2c4.com/wireguard/windows/protocol"
	"golang.zx2c4.com/wireguard/windows/services"
)

func TestWireHushServiceFailureMessagesDoNotExposeConfig(t *testing.T) {
	scenarios := []struct {
		code  services.Error
		match string
	}{
		{services.ErrorDNSLookup, "hostname"},
		{services.ErrorLoadConfiguration, "configuration"},
		{services.ErrorCreateNetworkAdapter, "adapter"},
		{services.ErrorFirewall, "firewall"},
		{services.ErrorBindSocketsToDefaultRoutes, "route"},
		{services.ErrorRunScript, "script"},
		{services.ErrorStartupBeforeNetwork, "startup"},
	}
	for _, scenario := range scenarios {
		status := svc.Status{
			Win32ExitCode:           uint32(windows.ERROR_SERVICE_SPECIFIC_ERROR),
			ServiceSpecificExitCode: uint32(scenario.code),
		}
		msg := wireHushServiceFailureMessage(status)
		if !strings.Contains(strings.ToLower(msg), scenario.match) {
			t.Errorf("exit code %d: unexpected diagnostic category: %q", scenario.code, msg)
		}
		if strings.Contains(msg, "PrivateKey") || strings.Contains(msg, "S-1-5") {
			t.Fatalf("diagnostic may contain sensitive material: %q", msg)
		}
	}
	if got := wireHushServiceFailureMessage(svc.Status{}); got != "" {
		t.Fatalf("successful status produced failure: %q", got)
	}
	if got := wireHushServiceFailureMessage(svc.Status{Win32ExitCode: uint32(windows.ERROR_ACCESS_DENIED)}); !strings.Contains(got, "Windows") {
		t.Fatalf("missing generic Win32 failure category: %q", got)
	}
}

func TestWireHushFailedStartupSnapshotIsAuthorizedAndPersistent(t *testing.T) {
	record := wireHushControlRecord(t, "12345678-1234-4abc-8def-1234567890ab", conf.TunnelScopePrivate, wireHushAuthOwnerSID, "Visible")
	other := wireHushControlRecord(t, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", conf.TunnelScopePrivate, wireHushAuthOtherSID, "SECRET")
	server, ctx, _ := testWireHushRPC(t, record, other)
	if _, err := server.Handshake(ctx, &protocol.HandshakeRequest{ProtocolMajor: 1}); err != nil {
		t.Fatal(err)
	}
	// Simulate the worker stopping after the Start RPC returned.
	setWireHushTunnelFailure(record.TunnelID, "Endpoint hostname resolution failed.")
	setWireHushTunnelFailure(other.TunnelID, "SECRET: other user's failure")
	t.Cleanup(func() {
		setWireHushTunnelFailure(record.TunnelID, "")
		setWireHushTunnelFailure(other.TunnelID, "")
	})
	server.mutations.control.state = func(conf.TunnelServiceLocator) (TunnelState, error) { return TunnelStopped, nil }
	for i := 0; i < 2; i++ {
		snapshot, err := server.Snapshot(ctx, &protocol.Empty{})
		if err != nil {
			t.Fatal(err)
		}
		if len(snapshot.Tunnels) != 1 || snapshot.Tunnels[0].LastFailure != "Endpoint hostname resolution failed." {
			t.Fatalf("failure was lost from authorized snapshot %v", snapshot)
		}
		if strings.Contains(snapshot.String(), "SECRET") {
			t.Fatal("a different account's failure appeared in snapshot")
		}
	}
	setWireHushTunnelFailure(record.TunnelID, "")
	snapshot, err := server.Snapshot(ctx, &protocol.Empty{})
	if err != nil || snapshot.Tunnels[0].LastFailure != "" {
		t.Fatalf("retry did not clear previous startup failure: %v, %v", snapshot, err)
	}
}

// Check that a failure is never confused with a successful, running tunnel.
func TestWireHushTrackedFailureNeverReportsActiveAsFailed(t *testing.T) {
	record := wireHushControlRecord(t, "12345678-1234-4abc-8def-1234567890ab", conf.TunnelScopePrivate, wireHushAuthOwnerSID, "Visible")
	server, ctx, _ := testWireHushRPC(t, record)
	if _, err := server.Handshake(ctx, &protocol.HandshakeRequest{ProtocolMajor: 1}); err != nil {
		t.Fatal(err)
	}
	setWireHushTunnelFailure(record.TunnelID, "Previous failure")
	defer setWireHushTunnelFailure(record.TunnelID, "")
	server.mutations.control.state = func(conf.TunnelServiceLocator) (TunnelState, error) { return TunnelStarted, nil }
	// Runtime config is not mocked in this test; a missing runtime yields
	// UNKNOWN instead of a false connected claim.
	snapshot, err := server.Snapshot(ctx, &protocol.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Tunnels[0].LastFailure != "" {
		t.Fatal("stale failure surfaced on running tunnel")
	}
}
