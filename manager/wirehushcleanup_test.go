/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */
package manager

import (
	"context"
	"errors"
	"golang.org/x/sys/windows/svc"
	"testing"
	"time"
)

func TestWireHushCleanupRetainsFailedWorkerService(t *testing.T) {
	for _, current := range []svc.Status{{State: svc.Stopped, Win32ExitCode: 1066, ServiceSpecificExitCode: 10}, {State: svc.Stopped, Win32ExitCode: 1}} {
		deleted := false
		err := removeWireHushTunnelAfterCleanup(context.Background(), func() (svc.Status, error) { return current, nil }, func() error { t.Fatal("stopped service was stopped again"); return nil }, func() error { deleted = true; return nil })
		if !errors.Is(err, errWireHushCleanupFailed) || deleted {
			t.Fatal("failed cleanup lost its service evidence")
		}
	}
}
func TestWireHushCleanupWaitsForVerifiedSuccessfulWorker(t *testing.T) {
	calls, stopped, deleted := 0, false, false
	err := removeWireHushTunnelAfterCleanup(context.Background(), func() (svc.Status, error) {
		calls++
		if calls < 3 {
			return svc.Status{State: svc.Running}, nil
		}
		return svc.Status{State: svc.Stopped}, nil
	}, func() error { stopped = true; return nil }, func() error { deleted = true; return nil })
	if err != nil || !stopped || !deleted || calls < 3 {
		t.Fatalf("cleanup stop=%v delete=%v calls=%d err=%v", stopped, deleted, calls, err)
	}
}
func TestWireHushCleanupDeadlineNeverDeletesService(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	deleted := false
	err := removeWireHushTunnelAfterCleanup(ctx, func() (svc.Status, error) { return svc.Status{State: svc.StopPending}, nil }, func() error { return nil }, func() error { deleted = true; return nil })
	if !errors.Is(err, context.DeadlineExceeded) || deleted {
		t.Fatal("incomplete cleanup removed service")
	}
}
