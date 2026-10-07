/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package manager

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/windows/conf"
)

func TestWireHushRemovalCancellationAndFailures(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	probes := 0
	if err := pollWireHushTunnelRemoval(ctx, func() error { probes++; return nil }); !errors.Is(err, context.Canceled) || probes != 0 {
		t.Fatalf("canceled wait = %v, probes = %d", err, probes)
	}
	for _, expected := range []error{windows.ERROR_ACCESS_DENIED, errWireHushTunnelServiceOwnershipConflict} {
		if err := pollWireHushTunnelRemoval(context.Background(), func() error { return expected }); !errors.Is(err, expected) {
			t.Fatalf("SCM/ownership failure = %v, want %v", err, expected)
		}
	}
	if err := pollWireHushTunnelRemoval(context.Background(), func() error { return windows.ERROR_SERVICE_DOES_NOT_EXIST }); err != nil {
		t.Fatal(err)
	}
	for _, pending := range []error{nil, windows.ERROR_SERVICE_MARKED_FOR_DELETE} {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		err := pollWireHushTunnelRemoval(ctx, func() error { return pending })
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("pending wait = %v", err)
		}
	}
}

func TestWireHushDeletePreservesRecordUntilCleanup(t *testing.T) {
	record := wireHushControlRecord(t, "12345678-1234-4abc-8def-1234567890ab", conf.TunnelScopePrivate, wireHushAuthOwnerSID, "Home")
	owner := wireHushCaller{SID: wireHushAuthOwnerSID}
	locator := wireHushControlLocator(record)
	for _, failure := range []error{context.DeadlineExceeded, windows.ERROR_ACCESS_DENIED, errWireHushTunnelServiceOwnershipConflict} {
		control, records, _ := wireHushControlForRecords(record)
		control.waitForStop = func(conf.TunnelServiceLocator) error { return failure }
		if err := control.DeleteTunnelRecord(owner, locator); !errors.Is(err, failure) {
			t.Fatalf("delete failure = %v, want %v", err, failure)
		}
		if len(*records) != 1 {
			t.Fatal("incomplete cleanup deleted protected record")
		}
	}
	control, records, _ := wireHushControlForRecords(record)
	waited := false
	control.stop = func(conf.TunnelServiceLocator) error { return windows.ERROR_SERVICE_DOES_NOT_EXIST }
	control.waitForStop = func(conf.TunnelServiceLocator) error { waited = true; return nil }
	if err := control.DeleteTunnelRecord(owner, locator); err != nil || !waited || len(*records) != 0 {
		t.Fatalf("already absent service delete = %v, waited = %v, records = %d", err, waited, len(*records))
	}
	control, _, _ = wireHushControlForRecords(record)
	control.stop = func(conf.TunnelServiceLocator) error { return windows.ERROR_ACCESS_DENIED }
	control.waitForStop = func(conf.TunnelServiceLocator) error { t.Fatal("wait ran after failed stop"); return nil }
	if err := control.StopTunnel(owner, locator); !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		t.Fatalf("stop error = %v", err)
	}
}
