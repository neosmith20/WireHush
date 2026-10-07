/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package manager

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"

	"golang.zx2c4.com/wireguard/windows/conf"
	"golang.zx2c4.com/wireguard/windows/product"
)

func wireHushLocatorFromServiceCommandLine(serviceName, binaryPathName, executablePath string) (conf.TunnelServiceLocator, bool, error) {
	args, err := windows.DecomposeCommandLine(binaryPathName)
	if err != nil {
		return conf.TunnelServiceLocator{}, false, err
	}
	if len(args) < 2 || args[1] != conf.WireHushTunnelServiceCommand {
		return conf.TunnelServiceLocator{}, false, nil
	}
	if !strings.EqualFold(args[0], executablePath) {
		return conf.TunnelServiceLocator{}, true, errors.New("WireHush tunnel service executable path does not match")
	}
	locator, err := conf.ParseWireHushTunnelServiceArgs(args[1:])
	if err != nil {
		return conf.TunnelServiceLocator{}, true, err
	}
	expectedServiceName, err := conf.ServiceNameOfTunnelID(locator.TunnelID)
	if err != nil {
		return conf.TunnelServiceLocator{}, true, err
	}
	if serviceName != expectedServiceName {
		return conf.TunnelServiceLocator{}, true, fmt.Errorf("WireHush tunnel service name %q does not match TunnelID identity %q", serviceName, expectedServiceName)
	}
	return locator, true, nil
}

func wireHushDiscoveredServiceShouldTrack(status svc.Status) bool {
	return status.State != svc.Stopped
}

func trackExistingWireHushTunnelServices() error {
	m, err := serviceManager()
	if err != nil {
		return err
	}
	executablePath, err := os.Executable()
	if err != nil {
		return err
	}
	serviceNames, err := m.ListServices()
	if err != nil {
		return err
	}
	for _, serviceName := range serviceNames {
		if !strings.HasPrefix(serviceName, product.TunnelServicePrefix) {
			continue
		}
		service, err := m.OpenService(serviceName)
		if err != nil {
			if err != windows.ERROR_SERVICE_DOES_NOT_EXIST && err != windows.ERROR_SERVICE_MARKED_FOR_DELETE {
				log.Printf("[%s] Unable to open candidate WireHush tunnel service: %v", serviceName, err)
			}
			continue
		}
		config, err := service.Config()
		if err != nil {
			service.Close()
			if err != windows.ERROR_SERVICE_MARKED_FOR_DELETE {
				log.Printf("[%s] Unable to read candidate WireHush tunnel service configuration: %v", serviceName, err)
			}
			continue
		}
		locator, recognized, err := wireHushLocatorFromServiceCommandLine(serviceName, config.BinaryPathName, executablePath)
		if err != nil {
			service.Close()
			log.Printf("[%s] Invalid WireHush tunnel service identity: %v", serviceName, err)
			continue
		}
		if !recognized {
			service.Close()
			continue
		}
		status, err := service.Query()
		if err := wireHushWorkerConfigMatches(config); err != nil {
			service.Close()
			log.Print("Rejected incompatible WireHush worker service binding")
			continue
		}
		if err != nil {
			service.Close()
			if err != windows.ERROR_SERVICE_MARKED_FOR_DELETE {
				log.Printf("[%s] Unable to query WireHush tunnel service: %v", serviceName, err)
			}
			continue
		}
		if !wireHushDiscoveredServiceShouldTrack(status) {
			service.Close()
			continue
		}
		go trackWireHushTunnelService(locator, service)
	}
	return nil
}
