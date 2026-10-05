package conf

import "testing"

func serviceIdentityTestID(t *testing.T) TunnelID {
	t.Helper()
	id, err := ParseTunnelID("12345678-1234-4abc-8def-1234567890ab")
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestTunnelServiceLocatorAndArgs(t *testing.T) {
	private := TunnelServiceLocator{Scope: TunnelScopePrivate, OwnerSID: "S-1-5-18", TunnelID: serviceIdentityTestID(t)}
	shared := TunnelServiceLocator{Scope: TunnelScopeShared, TunnelID: private.TunnelID}
	for _, test := range []struct {
		locator TunnelServiceLocator
		want    []string
	}{{private, []string{WireHushTunnelServiceCommand, "private", "S-1-5-18", private.TunnelID.String()}}, {shared, []string{WireHushTunnelServiceCommand, "shared", shared.TunnelID.String()}}} {
		args, err := WireHushTunnelServiceArgs(test.locator)
		if err != nil {
			t.Fatal(err)
		}
		if len(args) != len(test.want) {
			t.Fatalf("args = %#v", args)
		}
		for i := range args {
			if args[i] != test.want[i] {
				t.Fatalf("args = %#v", args)
			}
		}
		got, err := ParseWireHushTunnelServiceArgs(args)
		if err != nil || got != test.locator {
			t.Fatalf("roundtrip = %#v, %v", got, err)
		}
	}
}

func TestTunnelServiceLocatorRejectsInvalidArgs(t *testing.T) {
	const validID = "12345678-1234-4abc-8def-1234567890ab"
	for name, args := range map[string][]string{
		"wrong command":              {"/wrong", "shared", validID},
		"command case variant":       {"/WireHushTunnelService", "shared", validID},
		"private scope case variant": {WireHushTunnelServiceCommand, "Private", "S-1-5-18", validID},
		"private scope uppercase":    {WireHushTunnelServiceCommand, "PRIVATE", "S-1-5-18", validID},
		"shared scope case variant":  {WireHushTunnelServiceCommand, "Shared", validID},
		"shared scope uppercase":     {WireHushTunnelServiceCommand, "SHARED", validID},
		"unknown scope":              {WireHushTunnelServiceCommand, "other", validID},
		"private missing SID":        {WireHushTunnelServiceCommand, "private", validID},
		"private malformed SID":      {WireHushTunnelServiceCommand, "private", "not-a-sid", validID},
		"private noncanonical SID":   {WireHushTunnelServiceCommand, "private", "S-1-5-018", validID},
		"shared extra owner":         {WireHushTunnelServiceCommand, "shared", "S-1-5-18", validID},
		"private missing ID":         {WireHushTunnelServiceCommand, "private", "S-1-5-18"},
		"shared missing ID":          {WireHushTunnelServiceCommand, "shared"},
		"private extra argument":     {WireHushTunnelServiceCommand, "private", "S-1-5-18", validID, "extra"},
		"shared extra argument":      {WireHushTunnelServiceCommand, "shared", validID, "extra"},
		"uppercase ID":               {WireHushTunnelServiceCommand, "shared", "12345678-1234-4ABC-8DEF-1234567890AB"},
		"malformed ID":               {WireHushTunnelServiceCommand, "shared", "not-an-id"},
		"zero ID":                    {WireHushTunnelServiceCommand, "shared", "00000000-0000-0000-0000-000000000000"},
		"non-v4 UUID":                {WireHushTunnelServiceCommand, "shared", "12345678-1234-1abc-8def-1234567890ab"},
		"malformed argument count":   {WireHushTunnelServiceCommand},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseWireHushTunnelServiceArgs(args); err == nil {
				t.Fatalf("ParseWireHushTunnelServiceArgs(%q) unexpectedly succeeded", args)
			}
		})
	}
	if _, err := ServiceNameOfTunnelID(TunnelID{}); err == nil {
		t.Fatal("zero ID service name accepted")
	}
	name, err := ServiceNameOfTunnelID(serviceIdentityTestID(t))
	if err != nil || name != "WireHushTunnel$1234567812344abc8def1234567890ab" {
		t.Fatalf("service name = %q, %v", name, err)
	}
}
