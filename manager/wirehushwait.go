/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package manager

import (
	"context"
	"errors"
	"time"

	"golang.org/x/sys/windows"
)

const wireHushOperationTimeout = 10 * time.Second

// probe returns nil while the exact owned service exists, marked-for-delete
// while it is being removed, or DOES_NOT_EXIST once cleanup really completes.
// Ownership conflicts and genuine SCM failures are never treated as absence.
func pollWireHushTunnelRemoval(ctx context.Context, probe func() error) error {
	ctx, cancel := context.WithTimeout(ctx, wireHushOperationTimeout)
	defer cancel()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := probe()
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return nil
		}
		if err != nil && !errors.Is(err, windows.ERROR_SERVICE_MARKED_FOR_DELETE) {
			return err
		}
		timer := time.NewTimer(time.Second / 3)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
