/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush contributors. All Rights Reserved.
 */

package manager

import (
	"errors"
	"testing"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"golang.zx2c4.com/wireguard/windows/conf"
	"golang.zx2c4.com/wireguard/windows/product"
)

func TestManagerServiceConfigUsesDemandStart(t *testing.T) {
	config := managerServiceConfig()
	if config.StartType != mgr.StartManual {
		t.Fatalf("manager StartType = %d, want %d", config.StartType, mgr.StartManual)
	}
	if config.StartType == mgr.StartAutomatic {
		t.Fatal("manager StartType must not be automatic")
	}
	if config.DisplayName != product.ManagerServiceDisplayName {
		t.Fatalf("manager DisplayName = %q, want %q", config.DisplayName, product.ManagerServiceDisplayName)
	}
	if config.ServiceType != windows.SERVICE_WIN32_OWN_PROCESS {
		t.Fatalf("manager ServiceType = %d, want %d", config.ServiceType, windows.SERVICE_WIN32_OWN_PROCESS)
	}
	if config.ErrorControl != mgr.ErrorNormal {
		t.Fatalf("manager ErrorControl = %d, want %d", config.ErrorControl, mgr.ErrorNormal)
	}
}

func wireHushTunnelTestLocator(t *testing.T, scope conf.TunnelScope) conf.TunnelServiceLocator {
	t.Helper()
	id, err := conf.ParseTunnelID("12345678-1234-4abc-8def-1234567890ab")
	if err != nil {
		t.Fatal(err)
	}
	locator := conf.TunnelServiceLocator{Scope: scope, TunnelID: id}
	if scope == conf.TunnelScopePrivate {
		locator.OwnerSID = "S-1-5-18"
	}
	return locator
}

func TestWireHushTunnelServiceConfig(t *testing.T) {
	record := conf.TunnelRecord{Name: "Home"}
	config := wireHushTunnelServiceConfig(record)
	if config.StartType != mgr.StartManual {
		t.Fatalf("StartType = %d, want %d", config.StartType, mgr.StartManual)
	}
	if config.StartType == mgr.StartAutomatic {
		t.Fatal("WireHush tunnel StartType must not be automatic")
	}
	if config.ServiceType != windows.SERVICE_WIN32_OWN_PROCESS {
		t.Fatalf("ServiceType = %d, want %d", config.ServiceType, windows.SERVICE_WIN32_OWN_PROCESS)
	}
	if config.ErrorControl != mgr.ErrorNormal {
		t.Fatalf("ErrorControl = %d, want %d", config.ErrorControl, mgr.ErrorNormal)
	}
	if len(config.Dependencies) != 2 || config.Dependencies[0] != "Nsi" || config.Dependencies[1] != "TcpIp" {
		t.Fatalf("Dependencies = %#v", config.Dependencies)
	}
	if config.SidType != windows.SERVICE_SID_TYPE_UNRESTRICTED {
		t.Fatalf("SidType = %d, want %d", config.SidType, windows.SERVICE_SID_TYPE_UNRESTRICTED)
	}
}

func TestWireHushTunnelServiceIdentityUsesLocator(t *testing.T) {
	private := wireHushTunnelTestLocator(t, conf.TunnelScopePrivate)
	shared := wireHushTunnelTestLocator(t, conf.TunnelScopeShared)
	privateName, privateArgs, err := wireHushTunnelServiceIdentity(private)
	if err != nil {
		t.Fatal(err)
	}
	sharedName, sharedArgs, err := wireHushTunnelServiceIdentity(shared)
	if err != nil {
		t.Fatal(err)
	}
	wantName := "WireHushTunnel$1234567812344abc8def1234567890ab"
	if privateName != wantName || sharedName != wantName {
		t.Fatalf("service names = %q, %q", privateName, sharedName)
	}
	wantPrivateArgs := []string{"/wirehushtunnelservice", "private", "S-1-5-18", "12345678-1234-4abc-8def-1234567890ab"}
	wantSharedArgs := []string{"/wirehushtunnelservice", "shared", "12345678-1234-4abc-8def-1234567890ab"}
	if len(privateArgs) != len(wantPrivateArgs) || len(sharedArgs) != len(wantSharedArgs) {
		t.Fatalf("arguments = %#v, %#v", privateArgs, sharedArgs)
	}
	for i := range wantPrivateArgs {
		if privateArgs[i] != wantPrivateArgs[i] {
			t.Fatalf("private arguments = %#v", privateArgs)
		}
	}
	for i := range wantSharedArgs {
		if sharedArgs[i] != wantSharedArgs[i] {
			t.Fatalf("shared arguments = %#v", sharedArgs)
		}
	}
}

func TestWireHushTunnelServiceIdentityIgnoresMutableRecordName(t *testing.T) {
	locator := wireHushTunnelTestLocator(t, conf.TunnelScopePrivate)
	serviceName, args, err := wireHushTunnelServiceIdentity(locator)
	if err != nil {
		t.Fatal(err)
	}
	home := wireHushTunnelServiceConfig(conf.TunnelRecord{TunnelID: locator.TunnelID, Scope: locator.Scope, OwnerSID: locator.OwnerSID, Name: "Home"})
	office := wireHushTunnelServiceConfig(conf.TunnelRecord{TunnelID: locator.TunnelID, Scope: locator.Scope, OwnerSID: locator.OwnerSID, Name: "Office"})
	serviceNameAfterRename, argsAfterRename, err := wireHushTunnelServiceIdentity(locator)
	if err != nil {
		t.Fatal(err)
	}
	if serviceName != serviceNameAfterRename {
		t.Fatal("mutable record name changed service identity")
	}
	if len(args) != len(argsAfterRename) {
		t.Fatalf("arguments changed from %#v to %#v", args, argsAfterRename)
	}
	for i := range args {
		if args[i] != argsAfterRename[i] {
			t.Fatalf("arguments changed from %#v to %#v", args, argsAfterRename)
		}
	}
	if home.DisplayName != office.DisplayName {
		t.Fatal("SCM display name exposed a mutable record name")
	}
}

func TestWireHushTunnelServiceIdentityRejectsInvalidLocator(t *testing.T) {
	if _, _, err := wireHushTunnelServiceIdentity(conf.TunnelServiceLocator{}); err == nil {
		t.Fatal("invalid locator accepted")
	}
}

func TestWireHushTunnelStateFromServiceStatus(t *testing.T) {
	for _, test := range []struct {
		state svc.State
		want  TunnelState
	}{
		{svc.Stopped, TunnelStopped},
		{svc.StopPending, TunnelStopping},
		{svc.Running, TunnelStarted},
		{svc.StartPending, TunnelStarting},
		{svc.Paused, TunnelUnknown},
	} {
		if got := wireHushTunnelStateFromServiceStatus(svc.Status{State: test.state}); got != test.want {
			t.Fatalf("state %d = %d, want %d", test.state, got, test.want)
		}
	}
}

func TestWireHushTunnelUninstallResult(t *testing.T) {
	stopErr := errors.New("stop failure")
	deleteErr := errors.New("delete failure")
	for _, test := range []struct {
		name      string
		stopErr   error
		deleteErr error
		want      error
	}{
		{"both nil", nil, nil, nil},
		{"stop inactive", windows.ERROR_SERVICE_NOT_ACTIVE, nil, nil},
		{"stop marked for delete", windows.ERROR_SERVICE_MARKED_FOR_DELETE, nil, nil},
		{"stop error", stopErr, nil, stopErr},
		{"delete marked for delete", nil, windows.ERROR_SERVICE_MARKED_FOR_DELETE, nil},
		{"delete overrides stop", stopErr, deleteErr, deleteErr},
		{"delete error", nil, deleteErr, deleteErr},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := wireHushTunnelUninstallResult(test.stopErr, test.deleteErr); !errors.Is(got, test.want) {
				t.Fatalf("result = %v, want %v", got, test.want)
			}
		})
	}
}
