/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package tunnel

import (
	"errors"
	"golang.org/x/sys/windows"
	"testing"
)

func TestWireHushConnectionAliasCollisionPreservesRuntimeName(t *testing.T) {
	calls := 0
	alias, err := wireHushConnectionAlias("WireHush-private-id", func() error {
		calls++
		return windows.ERROR_DUP_NAME
	})
	if alias != "WireHush-private-id" || !errors.Is(err, windows.ERROR_DUP_NAME) || calls != 1 {
		t.Fatalf("collision must retain current alias with no retry: %q, %v, calls=%d", alias, err, calls)
	}
}

func TestWireHushConnectionAliasSuccess(t *testing.T) {
	alias, err := wireHushConnectionAlias("WireHush-private-id", func() error { return nil })
	if err != nil || alias != wireHushConnectionName {
		t.Fatalf("alias=%q, err=%v", alias, err)
	}
}
