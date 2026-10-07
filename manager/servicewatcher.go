/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2019-2026 WireGuard LLC. All Rights Reserved.
 */

package manager

import (
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
)

type serviceSubscriptionState struct {
	service  *mgr.Service
	cb       func(status uint32) bool
	done     sync.WaitGroup
	lock     sync.Mutex
	finished bool
}

// Serialize the initial query with Windows callbacks. Completion and callback
// state must be observed exactly once even if SCM notifications race startup.
func (state *serviceSubscriptionState) notify(notification uint32) bool {
	state.lock.Lock()
	defer state.lock.Unlock()
	if state.finished {
		return true
	}
	if notification == 0 {
		status, err := state.service.Query()
		if err == nil {
			notification = svcStateToNotifyState(uint32(status.State))
		}
	}
	if state.cb(notification) {
		state.finished = true
		state.done.Done()
	}
	return state.finished
}

var serviceSubscriptionCallbackPtr = windows.NewCallback(func(notification uint32, context uintptr) uintptr {
	state := (*serviceSubscriptionState)(unsafe.Pointer(context))
	state.notify(notification)
	return 0
})

func svcStateToNotifyState(s uint32) uint32 {
	switch s {
	case windows.SERVICE_STOPPED:
		return windows.SERVICE_NOTIFY_STOPPED
	case windows.SERVICE_START_PENDING:
		return windows.SERVICE_NOTIFY_START_PENDING
	case windows.SERVICE_STOP_PENDING:
		return windows.SERVICE_NOTIFY_STOP_PENDING
	case windows.SERVICE_RUNNING:
		return windows.SERVICE_NOTIFY_RUNNING
	case windows.SERVICE_CONTINUE_PENDING:
		return windows.SERVICE_NOTIFY_CONTINUE_PENDING
	case windows.SERVICE_PAUSE_PENDING:
		return windows.SERVICE_NOTIFY_PAUSE_PENDING
	case windows.SERVICE_PAUSED:
		return windows.SERVICE_NOTIFY_PAUSED
	case windows.SERVICE_NO_CHANGE:
		return 0
	default:
		return 0
	}
}

func notifyStateToTunState(s uint32) TunnelState {
	if s&(windows.SERVICE_NOTIFY_STOPPED|windows.SERVICE_NOTIFY_DELETED) != 0 {
		return TunnelStopped
	} else if s&(windows.SERVICE_NOTIFY_DELETE_PENDING|windows.SERVICE_NOTIFY_STOP_PENDING) != 0 {
		return TunnelStopping
	} else if s&windows.SERVICE_NOTIFY_RUNNING != 0 {
		return TunnelStarted
	} else if s&windows.SERVICE_NOTIFY_START_PENDING != 0 {
		return TunnelStarting
	} else {
		return TunnelUnknown
	}
}

func trackService(service *mgr.Service, callback func(status uint32) bool) error {
	var subscription uintptr
	state := &serviceSubscriptionState{service: service, cb: callback}
	state.done.Add(1)
	err := windows.SubscribeServiceChangeNotifications(service.Handle, windows.SC_EVENT_STATUS_CHANGE, serviceSubscriptionCallbackPtr, uintptr(unsafe.Pointer(state)), &subscription)
	if err != nil {
		return err
	}
	defer runtime.KeepAlive(state)
	defer windows.UnsubscribeServiceChangeNotifications(subscription)
	status, err := service.Query()
	if err == nil {
		if state.notify(svcStateToNotifyState(uint32(status.State))) {
			return nil
		}
	}
	state.done.Wait()
	return nil
}

func legacyTunnelServiceCommandLineRecognized(binaryPathName string) (bool, error) {
	args, err := windows.DecomposeCommandLine(binaryPathName)
	if err != nil {
		return false, err
	}
	return len(args) >= 2 && args[1] == "/tunnelservice", nil
}
