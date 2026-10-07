/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package manager

import (
	"errors"
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

type wireHushTrackedTunnel struct {
	Locator conf.TunnelServiceLocator
	State   TunnelState
}

var (
	wireHushTrackedTunnels           = make(map[conf.TunnelID]wireHushTrackedTunnel)
	wireHushTrackedTunnelsLock       sync.Mutex
	errWireHushTunnelLocatorConflict = errors.New("WireHush TunnelID is already tracked with a different locator")
)

func registerWireHushTrackedTunnel(locator conf.TunnelServiceLocator) (bool, error) {
	if err := locator.Validate(); err != nil {
		return false, err
	}
	wireHushTrackedTunnelsLock.Lock()
	defer wireHushTrackedTunnelsLock.Unlock()
	existing, found := wireHushTrackedTunnels[locator.TunnelID]
	if found {
		if existing.Locator != locator {
			return false, errWireHushTunnelLocatorConflict
		}
		return false, nil
	}
	wireHushTrackedTunnels[locator.TunnelID] = wireHushTrackedTunnel{Locator: locator, State: TunnelUnknown}
	return true, nil
}

func setWireHushTrackedTunnelState(id conf.TunnelID, state TunnelState) {
	wireHushTrackedTunnelsLock.Lock()
	defer wireHushTrackedTunnelsLock.Unlock()
	tracked, found := wireHushTrackedTunnels[id]
	if !found {
		return
	}
	tracked.State = state
	wireHushTrackedTunnels[id] = tracked
}

func removeWireHushTrackedTunnel(id conf.TunnelID) {
	wireHushTrackedTunnelsLock.Lock()
	defer wireHushTrackedTunnelsLock.Unlock()
	delete(wireHushTrackedTunnels, id)
}

func wireHushTrackedTunnelsGlobalState() (state TunnelState) {
	state = TunnelStopped
	stopping := false
	wireHushTrackedTunnelsLock.Lock()
	defer wireHushTrackedTunnelsLock.Unlock()
	for _, tracked := range wireHushTrackedTunnels {
		switch tracked.State {
		case TunnelStarting:
			return TunnelStarting
		case TunnelStopping:
			stopping = true
		case TunnelStarted, TunnelUnknown:
			state = TunnelStarted
		}
	}
	if stopping {
		return TunnelStopping
	}
	return
}

func wireHushTunnelServiceExitError(status svc.Status) error {
	if status.Win32ExitCode == uint32(windows.ERROR_SERVICE_SPECIFIC_ERROR) {
		if err := services.Error(status.ServiceSpecificExitCode); err != services.ErrorSuccess {
			return err
		}
		return nil
	}
	switch status.Win32ExitCode {
	case uint32(windows.NO_ERROR), uint32(windows.ERROR_SERVICE_NEVER_STARTED):
		return nil
	default:
		return syscall.Errno(status.Win32ExitCode)
	}
}

func trackWireHushTunnelService(locator conf.TunnelServiceLocator, service *mgr.Service) {
	registered, err := registerWireHushTrackedTunnel(locator)
	if err != nil {
		log.Printf("[%s] Unable to track WireHush tunnel service: %v", locator.TunnelID.String(), err)
		service.Close()
		return
	}
	if !registered {
		service.Close()
		return
	}
	serviceName, _ := conf.ServiceNameOfTunnelID(locator.TunnelID)
	defer func() {
		service.Close()
		removeWireHushTrackedTunnel(locator.TunnelID)
		log.Printf("[%s] WireHush tunnel service tracker finished", serviceName)
	}()

	for i := range 20 {
		if i > 0 {
			time.Sleep(time.Second / 5)
		}
		if status, err := service.Query(); err != nil || status.State != svc.Stopped {
			break
		}
	}

	checkForDisabled := func() bool {
		config, err := service.Config()
		if err == windows.ERROR_SERVICE_MARKED_FOR_DELETE || (err == nil && config.StartType == windows.SERVICE_DISABLED) {
			log.Printf("[%s] Disabled or externally removed WireHush tunnel service; preserving cleanup evidence", serviceName)
			setWireHushTrackedTunnelState(locator.TunnelID, TunnelUnknown)
			return true
		}
		return false
	}
	if checkForDisabled() {
		return
	}

	lastState := TunnelUnknown
	err = trackService(service, func(status uint32) bool {
		state := notifyStateToTunState(status)
		var tunnelError error
		if state == TunnelStopped {
			if serviceStatus, queryErr := service.Query(); queryErr == nil {
				tunnelError = wireHushTunnelServiceExitError(serviceStatus)
			}
			// Failed worker services retain their exit code until an administrator
			// repairs networking; deleting them would erase cleanup evidence.
		}
		if state != lastState {
			setWireHushTrackedTunnelState(locator.TunnelID, state)
			if tunnelError != nil {
				log.Printf("[%s] WireHush tunnel service failed: %v", serviceName, tunnelError)
			}
			lastState = state
		}
		if state == TunnelUnknown && checkForDisabled() {
			return true
		}
		return state == TunnelStopped
	})
	if err != nil && !checkForDisabled() {
		setWireHushTrackedTunnelState(locator.TunnelID, TunnelStopped)
		log.Printf("[%s] Unable to continue monitoring WireHush tunnel service, so stopping: %v", serviceName, err)
		service.Control(svc.Stop)
	}
}
