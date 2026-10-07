/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package manager

import (
	"errors"
	"os"
	"strings"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/windows/product"
)

// SCM is authoritative for admission, including the window before a newly
// started service's asynchronous tracker registers. Unclassifiable owned-prefix
// services fail closed; they are never stopped implicitly or assumed inactive.
func inventoryWireHushServices() ([]wireHushTrackedTunnel, error) {
	m, err := serviceManager()
	if err != nil {
		return nil, err
	}
	names, err := m.ListServices()
	if err != nil {
		return nil, err
	}
	path, err := os.Executable()
	if err != nil {
		return nil, err
	}
	result := []wireHushTrackedTunnel{}
	for _, name := range names {
		if !strings.HasPrefix(name, product.TunnelServicePrefix) {
			continue
		}
		service, err := m.OpenService(name)
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			continue
		}
		if errors.Is(err, windows.ERROR_SERVICE_MARKED_FOR_DELETE) {
			return nil, errWireHushDeviceBusy
		}
		if err != nil {
			return nil, err
		}
		config, configErr := service.Config()
		status, statusErr := service.Query()
		service.Close()
		if configErr != nil {
			return nil, configErr
		}
		if statusErr != nil {
			return nil, statusErr
		}
		locator, recognized, err := wireHushLocatorFromServiceCommandLine(name, config.BinaryPathName, path)
		if err != nil {
			return nil, err
		}
		state := wireHushTunnelStateFromServiceStatus(status)
		if !recognized {
			if state != TunnelStopped {
				return nil, errWireHushDeviceBusy
			}
			continue
		}
		result = append(result, wireHushTrackedTunnel{Locator: locator, State: state})
		if err := wireHushWorkerConfigMatches(config); err != nil {
			return nil, err
		}
	}
	return result, nil
}
