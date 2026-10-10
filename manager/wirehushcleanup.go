/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */
package manager

import (
	"context"
	"errors"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"time"
)

var errWireHushCleanupFailed = errors.New("WireHush cleanup requires administrator repair")

// Retain the SCM service and record when a worker reports failed cleanup. The
// service exit code must be observed before deleting the only durable evidence.
func removeWireHushTunnelAfterCleanup(ctx context.Context, query func() (svc.Status, error), stop, remove func() error) error {
	ctx, cancel := context.WithTimeout(ctx, wireHushOperationTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	current, err := query()
	if err != nil {
		return err
	}
	if current.State != svc.Stopped && current.State != svc.StopPending {
		if err := stop(); err != nil && !errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
			return err
		}
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		current, err = query()
		if err != nil {
			return err
		}
		if current.State == svc.Stopped {
			if wireHushTunnelNeedsRepair(current) {
				return errWireHushCleanupFailed
			}
			err := remove()
			if errors.Is(err, windows.ERROR_SERVICE_MARKED_FOR_DELETE) {
				return nil
			}
			return err
		}
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
