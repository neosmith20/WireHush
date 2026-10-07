/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package manager

import (
	"errors"
	"testing"

	"golang.org/x/sys/windows"

	"golang.zx2c4.com/wireguard/windows/conf"
)

func TestWireHushServiceCommandMatchesLocator(t *testing.T) {
	id := wireHushDiscoveryTestID(t)
	serviceName := wireHushDiscoveryTestServiceName(t, id)
	private := conf.TunnelServiceLocator{Scope: conf.TunnelScopePrivate, OwnerSID: "S-1-5-18", TunnelID: id}
	shared := conf.TunnelServiceLocator{Scope: conf.TunnelScopeShared, TunnelID: id}
	privateCommand := wireHushDiscoveryTestCommandLine(t, conf.WireHushTunnelServiceCommand, "private", "S-1-5-18", id.String())
	sharedCommand := wireHushDiscoveryTestCommandLine(t, conf.WireHushTunnelServiceCommand, "shared", id.String())
	legacyCollision := wireHushDiscoveryTestCommandLine(t, "/tunnelservice", `C:\legacy\1234567812344abc8def1234567890ab.conf`)

	for _, test := range []struct {
		name        string
		serviceName string
		command     string
		executable  string
		expected    conf.TunnelServiceLocator
		ownership   bool
	}{
		{"exact private", serviceName, privateCommand, wireHushDiscoveryTestExecutable, private, false},
		{"exact shared", serviceName, sharedCommand, wireHushDiscoveryTestExecutable, shared, false},
		{"different private owner", serviceName, privateCommand, wireHushDiscoveryTestExecutable, conf.TunnelServiceLocator{Scope: conf.TunnelScopePrivate, OwnerSID: "S-1-5-19", TunnelID: id}, true},
		{"different scope", serviceName, sharedCommand, wireHushDiscoveryTestExecutable, private, true},
		{"legacy collision", serviceName, legacyCollision, wireHushDiscoveryTestExecutable, private, true},
		{"unrelated command", serviceName, wireHushDiscoveryTestCommandLine(t, "/managerservice"), wireHushDiscoveryTestExecutable, private, true},
		{"malformed WireHush command", serviceName, wireHushDiscoveryTestCommandLine(t, conf.WireHushTunnelServiceCommand, "private", "S-1-5-18"), wireHushDiscoveryTestExecutable, private, false},
		{"wrong executable", serviceName, privateCommand, `C:\Other\wirehush.exe`, private, false},
		{"invalid expected locator", serviceName, privateCommand, wireHushDiscoveryTestExecutable, conf.TunnelServiceLocator{}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := wireHushServiceCommandMatchesLocator(test.serviceName, test.command, test.executable, test.expected)
			if test.name == "exact private" || test.name == "exact shared" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil {
				t.Fatal("ownership check unexpectedly succeeded")
			}
			if errors.Is(err, errWireHushTunnelServiceOwnershipConflict) != test.ownership {
				t.Fatalf("ownership conflict = %v, want %v (error: %v)", errors.Is(err, errWireHushTunnelServiceOwnershipConflict), test.ownership, err)
			}
		})
	}
}

func TestWireHushServiceCommandMatchesLocatorRejectsLegacyKeyCollision(t *testing.T) {
	id := wireHushDiscoveryTestID(t)
	serviceName := wireHushDiscoveryTestServiceName(t, id)
	if serviceName != "WireHushTunnel$1234567812344abc8def1234567890ab" {
		t.Fatalf("service name = %q", serviceName)
	}
	locator := conf.TunnelServiceLocator{Scope: conf.TunnelScopePrivate, OwnerSID: "S-1-5-18", TunnelID: id}
	legacyCommand := wireHushDiscoveryTestCommandLine(t, "/tunnelservice", `C:\legacy\1234567812344abc8def1234567890ab.conf`)
	err := wireHushServiceCommandMatchesLocator(serviceName, legacyCommand, wireHushDiscoveryTestExecutable, locator)
	if !errors.Is(err, errWireHushTunnelServiceOwnershipConflict) {
		t.Fatalf("legacy collision error = %v", err)
	}
}

func TestWireHushOwnershipConflictSentinel(t *testing.T) {
	if !errors.Is(errWireHushTunnelServiceOwnershipConflict, errWireHushTunnelServiceOwnershipConflict) {
		t.Fatal("ownership conflict sentinel is not errors.Is-compatible")
	}
	if errors.Is(windows.ERROR_SERVICE_DOES_NOT_EXIST, errWireHushTunnelServiceOwnershipConflict) {
		t.Fatal("service absence is an ownership conflict")
	}
}
