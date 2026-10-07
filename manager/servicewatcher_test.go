/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */
package manager

import (
	"golang.org/x/sys/windows"
	"sync"
	"testing"
)

func TestServiceNotificationCompletionSerializesInitialAndConcurrentCallbacks(t *testing.T) {
	calls := 0
	state := &serviceSubscriptionState{cb: func(uint32) bool { calls++; return true }}
	state.done.Add(1)
	var workers sync.WaitGroup
	for range 100 {
		workers.Go(func() { state.notify(windows.SERVICE_NOTIFY_STOPPED) })
	}
	workers.Wait()
	state.done.Wait()
	if calls != 1 {
		t.Fatalf("terminal callback executed %d times", calls)
	}
}
