/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush contributors. All Rights Reserved.
 */

package product

import "testing"

func TestRuntimeIdentityIsWireHushOwned(t *testing.T) {
	if Name != "WireHush" {
		t.Fatalf("unexpected product name %q", Name)
	}
	if ManagerServiceName == "WireGuardManager" {
		t.Fatal("manager service name collides with upstream WireGuard")
	}
	if TunnelServicePrefix == "WireGuardTunnel$" {
		t.Fatal("tunnel service prefix collides with upstream WireGuard")
	}
	if DataDirectoryName == "WireGuard" || AdminRegistryKey == `Software\WireGuard` {
		t.Fatal("persistent storage identity collides with upstream WireGuard")
	}
	if ManagerWindowClass == "WireGuard UI - Manage Tunnels" {
		t.Fatal("window identity collides with upstream WireGuard")
	}
}

func TestLegacyPersistentIdentityIsExplicit(t *testing.T) {
	if LegacyManagerServiceName != "TunnelMintManager" {
		t.Fatalf("unexpected legacy manager service name %q", LegacyManagerServiceName)
	}
	if LegacyTunnelServicePrefix != "TunnelMintTunnel$" {
		t.Fatalf("unexpected legacy tunnel service prefix %q", LegacyTunnelServicePrefix)
	}
	if ManagerServiceName != "WireHushManager" {
		t.Fatalf("unexpected manager service name %q", ManagerServiceName)
	}
	if TunnelServicePrefix != "WireHushTunnel$" {
		t.Fatalf("unexpected tunnel service prefix %q", TunnelServicePrefix)
	}
	if ManagerServiceName == LegacyManagerServiceName || TunnelServicePrefix == LegacyTunnelServicePrefix {
		t.Fatal("active service identity aliases a legacy service identity")
	}
	if ManagerServiceDisplayName != "WireHush Manager" {
		t.Fatalf("unexpected manager display name %q", ManagerServiceDisplayName)
	}
	if TunnelServiceDisplayPrefix != "WireHush Tunnel: " {
		t.Fatalf("unexpected tunnel service display prefix %q", TunnelServiceDisplayPrefix)
	}
	if DataDirectoryName != LegacyDataDirectoryName || AdminRegistryKey != LegacyAdminRegistryKey {
		t.Fatal("data compatibility identifiers changed")
	}
}
