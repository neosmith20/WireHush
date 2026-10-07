//go:build !wirehush_v1

/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2019-2026 WireGuard LLC. All Rights Reserved.
 */

package manager

import (
	"fmt"
	"log"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"golang.zx2c4.com/wireguard/windows/conf"
	"golang.zx2c4.com/wireguard/windows/services"
)

var (
	trackedTunnels     = make(map[string]TunnelState)
	trackedTunnelsLock = sync.Mutex{}
)

func trackedTunnelsGlobalState() (state TunnelState) {
	state = TunnelStopped
	trackedTunnelsLock.Lock()
	defer trackedTunnelsLock.Unlock()
	for _, s := range trackedTunnels {
		if s == TunnelStarting {
			return TunnelStarting
		} else if s == TunnelStopping {
			return TunnelStopping
		} else if s == TunnelStarted || s == TunnelUnknown {
			state = TunnelStarted
		}
	}
	return
}

func trackTunnelService(tunnelName string, service *mgr.Service) {
	trackedTunnelsLock.Lock()
	if _, found := trackedTunnels[tunnelName]; found {
		trackedTunnelsLock.Unlock()
		service.Close()
		return
	}

	defer func() {
		service.Close()
		log.Printf("[%s] Tunnel service tracker finished", tunnelName)
	}()
	trackedTunnels[tunnelName] = TunnelUnknown
	trackedTunnelsLock.Unlock()
	defer func() {
		trackedTunnelsLock.Lock()
		delete(trackedTunnels, tunnelName)
		trackedTunnelsLock.Unlock()
	}()

	for i := range 20 {
		if i > 0 {
			time.Sleep(time.Second / 5)
		}
		if status, err := service.Query(); err != nil || status.State != svc.Stopped {
			break
		}
	}

	checkForDisabled := func() (shouldReturn bool) {
		config, err := service.Config()
		if err == windows.ERROR_SERVICE_MARKED_FOR_DELETE || (err == nil && config.StartType == windows.SERVICE_DISABLED) {
			log.Printf("[%s] Found disabled service via timeout, so deleting", tunnelName)
			service.Delete()
			trackedTunnelsLock.Lock()
			trackedTunnels[tunnelName] = TunnelStopped
			trackedTunnelsLock.Unlock()
			releaseDriverAdapter(tunnelName)
			IPCServerNotifyTunnelChange(tunnelName, TunnelStopped, nil)
			return true
		}
		return false
	}
	if checkForDisabled() {
		return
	}
	lastState := TunnelUnknown
	err := trackService(service, func(status uint32) bool {
		state := notifyStateToTunState(status)
		var tunnelError error
		if state == TunnelStopped {
			serviceStatus, err := service.Query()
			if err == nil {
				if serviceStatus.Win32ExitCode == uint32(windows.ERROR_SERVICE_SPECIFIC_ERROR) {
					maybeErr := services.Error(serviceStatus.ServiceSpecificExitCode)
					if maybeErr != services.ErrorSuccess {
						tunnelError = maybeErr
					}
				} else {
					switch serviceStatus.Win32ExitCode {
					case uint32(windows.NO_ERROR), uint32(windows.ERROR_SERVICE_NEVER_STARTED):
					default:
						tunnelError = syscall.Errno(serviceStatus.Win32ExitCode)
					}
				}
			}
			if tunnelError != nil {
				service.Delete()
			}
		}
		if state != lastState {
			trackedTunnelsLock.Lock()
			trackedTunnels[tunnelName] = state
			trackedTunnelsLock.Unlock()
			if state == TunnelStopped {
				releaseDriverAdapter(tunnelName)
			}
			IPCServerNotifyTunnelChange(tunnelName, state, tunnelError)
			lastState = state
		}
		if state == TunnelUnknown && checkForDisabled() {
			return true
		}
		return state == TunnelStopped
	})
	if err != nil && !checkForDisabled() {
		trackedTunnelsLock.Lock()
		trackedTunnels[tunnelName] = TunnelStopped
		trackedTunnelsLock.Unlock()
		releaseDriverAdapter(tunnelName)
		IPCServerNotifyTunnelChange(tunnelName, TunnelStopped, fmt.Errorf("Unable to continue monitoring service, so stopping: %w", err))
		service.Control(svc.Stop)
	}
}

func trackExistingTunnels() error {
	m, err := serviceManager()
	if err != nil {
		return err
	}
	names, err := conf.ListConfigNames()
	if err != nil {
		return err
	}
	for _, name := range names {
		trackedTunnelsLock.Lock()
		if _, found := trackedTunnels[name]; found {
			trackedTunnelsLock.Unlock()
			continue
		}
		trackedTunnelsLock.Unlock()
		serviceName, err := conf.ServiceNameOfTunnel(name)
		if err != nil {
			continue
		}
		service, err := m.OpenService(serviceName)
		if err != nil {
			continue
		}
		config, err := service.Config()
		if err != nil {
			service.Close()
			log.Printf("[%s] Unable to read legacy tunnel service configuration: %v", name, err)
			continue
		}
		legacy, err := legacyTunnelServiceCommandLineRecognized(config.BinaryPathName)
		if err != nil {
			service.Close()
			log.Printf("[%s] Unable to classify legacy tunnel service command line: %v", name, err)
			continue
		}
		if !legacy {
			service.Close()
			continue
		}
		go trackTunnelService(name, service)
	}
	return nil
}

var servicesSubscriptionWatcherCallbackPtr = windows.NewCallback(func(notification uint32, context uintptr) uintptr {
	trackExistingTunnels()
	return 0
})

func watchNewTunnelServices() error {
	m, err := serviceManager()
	if err != nil {
		return err
	}
	var subscription uintptr
	err = windows.SubscribeServiceChangeNotifications(m.Handle, windows.SC_EVENT_DATABASE_CHANGE, servicesSubscriptionWatcherCallbackPtr, 0, &subscription)
	if err != nil {
		return err
	}
	// We probably could do:
	//     defer windows.UnsubscribeServiceChangeNotifications(subscription)
	// and then terminate after some point, but instead we just let this go forever; it's process-lived.
	return trackExistingTunnels()
}
