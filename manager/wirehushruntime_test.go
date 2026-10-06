/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package manager

import (
	"testing"

	"golang.zx2c4.com/wireguard/windows/conf"
)

const wireHushRuntimeTestWGQuick = "[Interface]\nPrivateKey = AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\nAddress = 10.0.0.2/32\n"

func wireHushRuntimeTestID(t *testing.T, text string) conf.TunnelID {
	t.Helper()
	id, err := conf.ParseTunnelID(text)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func wireHushRuntimeTestLocator(t *testing.T, scope conf.TunnelScope, owner string, id conf.TunnelID) conf.TunnelServiceLocator {
	t.Helper()
	return conf.TunnelServiceLocator{Scope: scope, OwnerSID: owner, TunnelID: id}
}

func wireHushRuntimeTestRecord(t *testing.T, name string, id conf.TunnelID) conf.TunnelRecord {
	t.Helper()
	return conf.TunnelRecord{TunnelID: id, Name: name, Scope: conf.TunnelScopePrivate, OwnerSID: "S-1-5-18", WGQuickText: wireHushRuntimeTestWGQuick}
}

func TestWireHushRuntimeAdapterNameUsesTunnelID(t *testing.T) {
	id := wireHushRuntimeTestID(t, "12345678-1234-4abc-8def-1234567890ab")
	private := wireHushRuntimeTestLocator(t, conf.TunnelScopePrivate, "S-1-5-18", id)
	shared := wireHushRuntimeTestLocator(t, conf.TunnelScopeShared, "", id)
	differentOwner := wireHushRuntimeTestLocator(t, conf.TunnelScopePrivate, "S-1-5-19", id)
	privateName, err := wireHushRuntimeAdapterName(private)
	if err != nil {
		t.Fatal(err)
	}
	for _, locator := range []conf.TunnelServiceLocator{shared, differentOwner} {
		name, err := wireHushRuntimeAdapterName(locator)
		if err != nil || name != privateName {
			t.Fatalf("adapter name = %q, %v", name, err)
		}
	}
	otherID := wireHushRuntimeTestID(t, "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee")
	otherName, err := wireHushRuntimeAdapterName(wireHushRuntimeTestLocator(t, conf.TunnelScopeShared, "", otherID))
	if err != nil {
		t.Fatal(err)
	}
	if otherName == privateName {
		t.Fatal("different TunnelIDs produced the same adapter name")
	}
	if _, err := wireHushRuntimeAdapterName(conf.TunnelServiceLocator{}); err == nil {
		t.Fatal("invalid locator accepted")
	}
}

func TestWireHushStoredConfigFromRecord(t *testing.T) {
	id := wireHushRuntimeTestID(t, "12345678-1234-4abc-8def-1234567890ab")
	home := wireHushRuntimeTestRecord(t, "Home", id)
	config, err := wireHushStoredConfigFromRecord(home)
	if err != nil {
		t.Fatal(err)
	}
	if config.Name != "Home" || len(config.Interface.Addresses) != 1 {
		t.Fatalf("stored config = %#v", config)
	}
	office := home
	office.Name = "Office"
	changed, err := wireHushStoredConfigFromRecord(office)
	if err != nil {
		t.Fatal(err)
	}
	if changed.Name != "Office" {
		t.Fatalf("changed config name = %q", changed.Name)
	}
	name, err := wireHushRuntimeAdapterName(wireHushRuntimeTestLocator(t, conf.TunnelScopePrivate, home.OwnerSID, home.TunnelID))
	if err != nil {
		t.Fatal(err)
	}
	changedName, err := wireHushRuntimeAdapterName(wireHushRuntimeTestLocator(t, conf.TunnelScopePrivate, office.OwnerSID, office.TunnelID))
	if err != nil || changedName != name {
		t.Fatalf("adapter name changed from %q to %q: %v", name, changedName, err)
	}
	malformed := home
	malformed.WGQuickText = "not a WireGuard configuration"
	if _, err := wireHushStoredConfigFromRecord(malformed); err == nil {
		t.Fatal("malformed WGQuickText accepted")
	}
}
