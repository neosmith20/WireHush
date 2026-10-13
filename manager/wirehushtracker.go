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
	wireHushTunnelFailures           = make(map[conf.TunnelID]string)
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

// Failure text is a fixed diagnostic category derived from the worker's
// Windows service exit code. Do not forward untrusted endpoint/config/log text.
func wireHushServiceFailureMessage(status svc.Status) string {
	if wireHushTunnelServiceExitError(status) == nil {
		return ""
	}
	if status.Win32ExitCode == uint32(windows.ERROR_SERVICE_SPECIFIC_ERROR) {
		switch services.Error(status.ServiceSpecificExitCode) {
		case services.ErrorDNSLookup:
			return "Endpoint hostname resolution failed. Check the hostname, network connection and DNS settings."
		case services.ErrorLoadConfiguration, services.ErrorDeviceSetConfig:
			return "WireGuard configuration could not be applied. Check tunnel settings and peer keys."
		case services.ErrorCreateNetworkAdapter, services.ErrorDeviceBringUp:
			return "WireGuard network adapter could not start. Check the driver and Windows networking."
		case services.ErrorFirewall:
			return "Windows firewall setup failed. Check security software and firewall permissions."
		case services.ErrorBindSocketsToDefaultRoutes, services.ErrorMonitorMTUChanges, services.ErrorSetNetConfig:
			return "Network route or adapter setup failed. Check the interface and routing configuration."
		case services.ErrorRunScript:
			return "A configured tunnel script failed. Check the script and permissions in diagnostics."
		case services.ErrorStartupBeforeNetwork:
			return "Tunnel startup failed before network activation. Check the configuration and retry."
		default:
			return "The tunnel service stopped with a startup error. Check WireHush diagnostics."
		}
	}
	return "Windows could not start the tunnel service. Check the service and WireHush diagnostics."
}

func setWireHushTunnelFailure(id conf.TunnelID, message string) {
	wireHushTrackedTunnelsLock.Lock()
	defer wireHushTrackedTunnelsLock.Unlock()
	if message == "" {
		delete(wireHushTunnelFailures, id)
	} else {
		wireHushTunnelFailures[id] = message
	}
}

func wireHushTunnelFailure(id conf.TunnelID) string {
	wireHushTrackedTunnelsLock.Lock()
	defer wireHushTrackedTunnelsLock.Unlock()
	return wireHushTunnelFailures[id]
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

func wireHushTunnelNeedsRepair(status svc.Status) bool {
	// Only the explicit pre-network marker permits retry after a failed exit.
	// Unknown failures and every failure after adapter creation stay fail-closed.
	return wireHushTunnelServiceExitError(status) != nil && !(status.Win32ExitCode == uint32(windows.ERROR_SERVICE_SPECIFIC_ERROR) && status.ServiceSpecificExitCode == uint32(services.ErrorStartupBeforeNetwork))
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
				if tunnelError != nil {
					setWireHushTunnelFailure(locator.TunnelID, wireHushServiceFailureMessage(serviceStatus))
				}
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
		// Losing an observer is not authorization to disrupt a live VPN. SCM
		// inventory remains authoritative; admission fails closed if it cannot
		// query that truth. Leave the independent worker running.
		setWireHushTrackedTunnelState(locator.TunnelID, TunnelUnknown)
		log.Print("tunnel-observer-unavailable")
	}
}
