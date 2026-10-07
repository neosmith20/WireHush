/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package manager

import (
	"strings"
	"testing"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"

	"golang.zx2c4.com/wireguard/windows/conf"
)

const wireHushDiscoveryTestExecutable = `C:\Program Files\WireHush\wirehush.exe`

func wireHushDiscoveryTestID(t *testing.T) conf.TunnelID {
	t.Helper()
	id, err := conf.ParseTunnelID("12345678-1234-4abc-8def-1234567890ab")
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func wireHushDiscoveryTestServiceName(t *testing.T, id conf.TunnelID) string {
	t.Helper()
	name, err := conf.ServiceNameOfTunnelID(id)
	if err != nil {
		t.Fatal(err)
	}
	return name
}

func wireHushDiscoveryTestCommandLine(t *testing.T, args ...string) string {
	t.Helper()
	return windows.ComposeCommandLine(append([]string{wireHushDiscoveryTestExecutable}, args...))
}

func TestWireHushLocatorFromServiceCommandLine(t *testing.T) {
	id := wireHushDiscoveryTestID(t)
	serviceName := wireHushDiscoveryTestServiceName(t, id)
	for _, test := range []struct {
		name string
		args []string
		want conf.TunnelServiceLocator
	}{
		{"private", []string{conf.WireHushTunnelServiceCommand, "private", "S-1-5-18", id.String()}, conf.TunnelServiceLocator{Scope: conf.TunnelScopePrivate, OwnerSID: "S-1-5-18", TunnelID: id}},
		{"shared", []string{conf.WireHushTunnelServiceCommand, "shared", id.String()}, conf.TunnelServiceLocator{Scope: conf.TunnelScopeShared, TunnelID: id}},
	} {
		t.Run(test.name, func(t *testing.T) {
			locator, recognized, err := wireHushLocatorFromServiceCommandLine(serviceName, wireHushDiscoveryTestCommandLine(t, test.args...), strings.ToUpper(wireHushDiscoveryTestExecutable))
			if err != nil || !recognized || locator != test.want {
				t.Fatalf("locator = %#v, recognized = %v, err = %v", locator, recognized, err)
			}
		})
	}
}

func TestWireHushLocatorFromServiceCommandLineRejectsLegacyCollision(t *testing.T) {
	id := wireHushDiscoveryTestID(t)
	serviceName := wireHushDiscoveryTestServiceName(t, id)
	locator, recognized, err := wireHushLocatorFromServiceCommandLine(serviceName, wireHushDiscoveryTestCommandLine(t, "/tunnelservice", `C:\legacy\1234567812344abc8def1234567890ab.conf`), wireHushDiscoveryTestExecutable)
	if err != nil || recognized || locator != (conf.TunnelServiceLocator{}) {
		t.Fatalf("locator = %#v, recognized = %v, err = %v", locator, recognized, err)
	}
}

func TestWireHushLocatorFromServiceCommandLineRejectsInvalidNewServices(t *testing.T) {
	id := wireHushDiscoveryTestID(t)
	serviceName := wireHushDiscoveryTestServiceName(t, id)
	for _, test := range []struct {
		name           string
		serviceName    string
		executablePath string
		args           []string
	}{
		{"wrong executable", serviceName, wireHushDiscoveryTestExecutable, []string{conf.WireHushTunnelServiceCommand, "private", "S-1-5-18", id.String()}},
		{"wrong service name", "WireHushTunnel$aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", wireHushDiscoveryTestExecutable, []string{conf.WireHushTunnelServiceCommand, "shared", id.String()}},
		{"malformed private", serviceName, wireHushDiscoveryTestExecutable, []string{conf.WireHushTunnelServiceCommand, "private", "S-1-5-18"}},
		{"malformed shared", serviceName, wireHushDiscoveryTestExecutable, []string{conf.WireHushTunnelServiceCommand, "shared", "S-1-5-18", id.String()}},
		{"invalid scope", serviceName, wireHushDiscoveryTestExecutable, []string{conf.WireHushTunnelServiceCommand, "other", id.String()}},
		{"malformed SID", serviceName, wireHushDiscoveryTestExecutable, []string{conf.WireHushTunnelServiceCommand, "private", "not-a-sid", id.String()}},
		{"uppercase ID", serviceName, wireHushDiscoveryTestExecutable, []string{conf.WireHushTunnelServiceCommand, "shared", "12345678-1234-4ABC-8DEF-1234567890AB"}},
		{"malformed ID", serviceName, wireHushDiscoveryTestExecutable, []string{conf.WireHushTunnelServiceCommand, "shared", "not-an-id"}},
		{"extra argument", serviceName, wireHushDiscoveryTestExecutable, []string{conf.WireHushTunnelServiceCommand, "shared", id.String(), "extra"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			binaryPath := wireHushDiscoveryTestCommandLine(t, test.args...)
			if test.name == "wrong executable" {
				binaryPath = windows.ComposeCommandLine(append([]string{`C:\Other\wirehush.exe`}, test.args...))
			}
			_, recognized, err := wireHushLocatorFromServiceCommandLine(test.serviceName, binaryPath, test.executablePath)
			if !recognized || err == nil {
				t.Fatalf("recognized = %v, err = %v", recognized, err)
			}
		})
	}
}

func TestWireHushDiscoveredServiceShouldTrack(t *testing.T) {
	for _, test := range []struct {
		state svc.State
		want  bool
	}{
		{svc.Stopped, false},
		{svc.Running, true},
		{svc.StartPending, true},
		{svc.StopPending, true},
	} {
		if got := wireHushDiscoveredServiceShouldTrack(svc.Status{State: test.state}); got != test.want {
			t.Fatalf("state %d: should track = %v, want %v", test.state, got, test.want)
		}
	}
}
