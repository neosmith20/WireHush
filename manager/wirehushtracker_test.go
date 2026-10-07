/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package manager

import (
	"errors"
	"syscall"
	"testing"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"

	"golang.zx2c4.com/wireguard/windows/conf"
	"golang.zx2c4.com/wireguard/windows/services"
)

func wireHushTrackerTestID(t *testing.T, text string) conf.TunnelID {
	t.Helper()
	id, err := conf.ParseTunnelID(text)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func wireHushTrackerTestLocator(t *testing.T, id conf.TunnelID, owner string) conf.TunnelServiceLocator {
	t.Helper()
	return conf.TunnelServiceLocator{Scope: conf.TunnelScopePrivate, OwnerSID: owner, TunnelID: id}
}

func withWireHushTracker(t *testing.T) {
	t.Helper()
	wireHushTrackedTunnelsLock.Lock()
	previous := wireHushTrackedTunnels
	wireHushTrackedTunnels = make(map[conf.TunnelID]wireHushTrackedTunnel)
	wireHushTrackedTunnelsLock.Unlock()
	t.Cleanup(func() {
		wireHushTrackedTunnelsLock.Lock()
		wireHushTrackedTunnels = previous
		wireHushTrackedTunnelsLock.Unlock()
	})
}

func TestWireHushTrackerRegistrationUsesTunnelID(t *testing.T) {
	withWireHushTracker(t)
	id := wireHushTrackerTestID(t, "12345678-1234-4abc-8def-1234567890ab")
	locator := wireHushTrackerTestLocator(t, id, "S-1-5-18")
	registered, err := registerWireHushTrackedTunnel(locator)
	if err != nil || !registered {
		t.Fatalf("registration = %v, %v", registered, err)
	}
	registered, err = registerWireHushTrackedTunnel(locator)
	if err != nil || registered {
		t.Fatalf("duplicate registration = %v, %v", registered, err)
	}
	conflicting := wireHushTrackerTestLocator(t, id, "S-1-5-19")
	if _, err := registerWireHushTrackedTunnel(conflicting); !errors.Is(err, errWireHushTunnelLocatorConflict) {
		t.Fatalf("conflicting registration error = %v", err)
	}
	wireHushTrackedTunnelsLock.Lock()
	tracked := wireHushTrackedTunnels[id]
	wireHushTrackedTunnelsLock.Unlock()
	if tracked.Locator != locator {
		t.Fatal("conflicting locator replaced the original locator")
	}
	if _, err := registerWireHushTrackedTunnel(conf.TunnelServiceLocator{}); err == nil {
		t.Fatal("invalid locator accepted")
	}
}

func TestWireHushTrackerKeepsDifferentTunnelIDsIndependent(t *testing.T) {
	withWireHushTracker(t)
	first := wireHushTrackerTestID(t, "12345678-1234-4abc-8def-1234567890ab")
	second := wireHushTrackerTestID(t, "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee")
	for _, id := range []conf.TunnelID{first, second} {
		if registered, err := registerWireHushTrackedTunnel(wireHushTrackerTestLocator(t, id, "S-1-5-18")); err != nil || !registered {
			t.Fatalf("registration = %v, %v", registered, err)
		}
	}
	removeWireHushTrackedTunnel(first)
	wireHushTrackedTunnelsLock.Lock()
	_, firstFound := wireHushTrackedTunnels[first]
	_, secondFound := wireHushTrackedTunnels[second]
	wireHushTrackedTunnelsLock.Unlock()
	if firstFound || !secondFound {
		t.Fatalf("removal affected wrong entries: first=%v second=%v", firstFound, secondFound)
	}
}

func TestWireHushTrackerGlobalStatePrecedence(t *testing.T) {
	withWireHushTracker(t)
	if state := wireHushTrackedTunnelsGlobalState(); state != TunnelStopped {
		t.Fatalf("empty state = %v", state)
	}
	first := wireHushTrackerTestID(t, "12345678-1234-4abc-8def-1234567890ab")
	second := wireHushTrackerTestID(t, "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee")
	setStates := func(firstState, secondState TunnelState) {
		wireHushTrackedTunnelsLock.Lock()
		wireHushTrackedTunnels = map[conf.TunnelID]wireHushTrackedTunnel{
			first:  {Locator: wireHushTrackerTestLocator(t, first, "S-1-5-18"), State: firstState},
			second: {Locator: wireHushTrackerTestLocator(t, second, "S-1-5-18"), State: secondState},
		}
		wireHushTrackedTunnelsLock.Unlock()
	}
	setStates(TunnelStopped, TunnelStopped)
	if state := wireHushTrackedTunnelsGlobalState(); state != TunnelStopped {
		t.Fatalf("stopped-only state = %v", state)
	}
	setStates(TunnelStarted, TunnelStopped)
	if state := wireHushTrackedTunnelsGlobalState(); state != TunnelStarted {
		t.Fatalf("started state = %v", state)
	}
	setStates(TunnelUnknown, TunnelStopped)
	if state := wireHushTrackedTunnelsGlobalState(); state != TunnelStarted {
		t.Fatalf("unknown state = %v", state)
	}
	setStates(TunnelStopping, TunnelStarted)
	if state := wireHushTrackedTunnelsGlobalState(); state != TunnelStopping {
		t.Fatalf("stopping plus started state = %v", state)
	}
	setStates(TunnelStarting, TunnelStopping)
	if state := wireHushTrackedTunnelsGlobalState(); state != TunnelStarting {
		t.Fatalf("starting plus stopping state = %v", state)
	}
}

func TestWireHushTrackerIdentityIgnoresRecordName(t *testing.T) {
	withWireHushTracker(t)
	id := wireHushTrackerTestID(t, "12345678-1234-4abc-8def-1234567890ab")
	home := conf.TunnelRecord{TunnelID: id, Name: "Home"}
	office := home
	office.Name = "Office"
	if home.TunnelID != office.TunnelID {
		t.Fatal("renaming changed TunnelID")
	}
	locator := wireHushTrackerTestLocator(t, home.TunnelID, "S-1-5-18")
	if registered, err := registerWireHushTrackedTunnel(locator); err != nil || !registered {
		t.Fatalf("home registration = %v, %v", registered, err)
	}
	if registered, err := registerWireHushTrackedTunnel(wireHushTrackerTestLocator(t, office.TunnelID, "S-1-5-18")); err != nil || registered {
		t.Fatalf("renamed registration = %v, %v", registered, err)
	}
	wireHushTrackedTunnelsLock.Lock()
	_, found := wireHushTrackedTunnels[id]
	count := len(wireHushTrackedTunnels)
	wireHushTrackedTunnelsLock.Unlock()
	if !found || count != 1 {
		t.Fatalf("tracked runtime identities: found=%v count=%d", found, count)
	}
}

func TestWireHushTunnelServiceExitError(t *testing.T) {
	for _, test := range []struct {
		status svc.Status
		want   error
	}{
		{svc.Status{Win32ExitCode: uint32(windows.NO_ERROR)}, nil},
		{svc.Status{Win32ExitCode: uint32(windows.ERROR_SERVICE_NEVER_STARTED)}, nil},
		{svc.Status{Win32ExitCode: uint32(windows.ERROR_SERVICE_SPECIFIC_ERROR), ServiceSpecificExitCode: uint32(services.ErrorLoadConfiguration)}, services.ErrorLoadConfiguration},
		{svc.Status{Win32ExitCode: 12345}, syscall.Errno(12345)},
	} {
		got := wireHushTunnelServiceExitError(test.status)
		if !errors.Is(got, test.want) {
			t.Fatalf("error = %v, want %v", got, test.want)
		}
	}
}
