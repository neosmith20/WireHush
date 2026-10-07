/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */
package manager

import (
	"context"
	"errors"
	"golang.zx2c4.com/wireguard/windows/conf"
	"testing"
)

func TestWireHushNonAdminCannotIntroducePrivilegedScriptHooks(t *testing.T) {
	owner := wireHushCaller{SID: wireHushAuthOwnerSID, WireHushUser: true}
	for _, hook := range []string{"PreUp", "PostUp", "PreDown", "PostDown"} {
		record := wireHushControlRecord(t, "12345678-1234-4abc-8def-1234567890ab", conf.TunnelScopePrivate, owner.SID, "Test")
		record.WGQuickText += hook + " = cmd.exe /c echo example\n"
		control, _, _ := wireHushControlForRecords(record)
		control.saveRecord = func(conf.TunnelRecord, bool) error { t.Fatal("non-admin script hook reached storage"); return nil }
		control.start = func(conf.TunnelServiceLocator) error {
			t.Fatal("non-admin script hook reached service activation")
			return nil
		}
		locator := wireHushControlLocator(record)
		for _, operation := range []func() error{func() error { return control.CreateTunnelRecord(owner, record) }, func() error { return control.SaveTunnelRecord(owner, locator, record) }, func() error { return control.StartTunnel(owner, locator) }} {
			if err := operation(); !errors.Is(err, errWireHushAccessDenied) {
				t.Fatalf("hook=%s error=%v", hook, err)
			}
		}
		mutations := testWireHushMutations(record)
		if err := mutations.Start(context.Background(), owner, locator); !errors.Is(err, errWireHushAccessDenied) {
			t.Fatal(err)
		}
	}
}

func TestWireHushControlUsesRemainingCleanupDeadline(t *testing.T) {
	control, _, _ := wireHushControlForRecords()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	control.waitContext = func(actual context.Context, _ conf.TunnelServiceLocator) error {
		if actual != ctx {
			t.Fatal("cleanup replaced caller deadline")
		}
		return actual.Err()
	}
	if err := control.withContext(ctx).waitForStop(conf.TunnelServiceLocator{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
