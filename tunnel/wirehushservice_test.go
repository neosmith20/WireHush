package tunnel

import (
	"strings"
	"testing"

	"golang.zx2c4.com/wireguard/windows/conf"
)

func wireHushServiceTestTunnelID(t *testing.T) conf.TunnelID {
	t.Helper()
	id, err := conf.ParseTunnelID("12345678-1234-4abc-8def-1234567890ab")
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func wireHushServiceTestRecord(t *testing.T, name string, id conf.TunnelID) conf.TunnelRecord {
	t.Helper()
	return conf.TunnelRecord{TunnelID: id, Name: name, Scope: conf.TunnelScopePrivate, OwnerSID: "S-1-5-18", WGQuickText: "[Interface]\nPrivateKey = AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\nAddress = 10.0.0.2/32\n"}
}
func TestWireHushRuntimeFromRecord(t *testing.T) {
	id := wireHushServiceTestTunnelID(t)
	record := wireHushServiceTestRecord(t, "Home", id)
	config, identity, err := wireHushRuntimeFromRecord(record)
	if err != nil || config.Name != "Home" {
		t.Fatalf("runtime = %#v, %v", config, err)
	}
	renamed := record
	renamed.Name = "Office"
	changed, sameIdentity, err := wireHushRuntimeFromRecord(renamed)
	if err != nil || changed.Name != "Office" || sameIdentity != identity {
		t.Fatalf("rename runtime = %#v %#v %v", changed, sameIdentity, err)
	}
	if _, _, err := wireHushRuntimeFromRecord(conf.TunnelRecord{}); err == nil {
		t.Fatal("invalid record accepted")
	}
}

func TestWireHushRuntimeFromRecordUsesCurrentWGQuickText(t *testing.T) {
	id := wireHushServiceTestTunnelID(t)
	first := wireHushServiceTestRecord(t, "Home", id)
	second := first
	second.WGQuickText = strings.Replace(second.WGQuickText, "10.0.0.2/32", "10.0.0.3/32", 1)

	firstConfig, firstIdentity, err := wireHushRuntimeFromRecord(first)
	if err != nil {
		t.Fatal(err)
	}
	secondConfig, secondIdentity, err := wireHushRuntimeFromRecord(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstConfig.ToWgQuick() == secondConfig.ToWgQuick() {
		t.Fatal("runtime config did not reflect changed WGQuickText")
	}
	if firstIdentity != secondIdentity {
		t.Fatal("mutable configuration changed adapter identity")
	}
}

func TestWireHushRuntimeFromRecordScopeAndOwnerDoNotChangeIdentity(t *testing.T) {
	id := wireHushServiceTestTunnelID(t)
	private := wireHushServiceTestRecord(t, "Home", id)
	shared := private
	shared.Scope = conf.TunnelScopeShared
	shared.OwnerSID = ""

	_, privateIdentity, err := wireHushRuntimeFromRecord(private)
	if err != nil {
		t.Fatal(err)
	}
	_, sharedIdentity, err := wireHushRuntimeFromRecord(shared)
	if err != nil {
		t.Fatal(err)
	}
	if privateIdentity != sharedIdentity {
		t.Fatal("scope or owner changed adapter identity")
	}
}

func TestWireHushRuntimeFromRecordDifferentIDsChangeIdentity(t *testing.T) {
	firstID := wireHushServiceTestTunnelID(t)
	secondID, err := conf.ParseTunnelID("abcdefab-cdef-4abc-8def-1234567890ab")
	if err != nil {
		t.Fatal(err)
	}
	first := wireHushServiceTestRecord(t, "Home", firstID)
	second := first
	second.TunnelID = secondID

	_, firstIdentity, err := wireHushRuntimeFromRecord(first)
	if err != nil {
		t.Fatal(err)
	}
	_, secondIdentity, err := wireHushRuntimeFromRecord(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstIdentity == secondIdentity {
		t.Fatal("different TunnelIDs produced the same adapter identity")
	}
}

func TestWireHushRuntimeFromRecordRejectsMalformedWGQuickText(t *testing.T) {
	record := wireHushServiceTestRecord(t, "Home", wireHushServiceTestTunnelID(t))
	record.WGQuickText = "this is not a WireGuard configuration"
	if _, _, err := wireHushRuntimeFromRecord(record); err == nil {
		t.Fatal("wireHushRuntimeFromRecord unexpectedly accepted malformed WGQuickText")
	}
}
