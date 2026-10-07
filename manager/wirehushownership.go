/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package manager

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows/svc/mgr"

	"golang.zx2c4.com/wireguard/windows/conf"
)

var errWireHushTunnelServiceOwnershipConflict = errors.New("WireHush tunnel service ownership conflict")

func wireHushServiceCommandMatchesLocator(serviceName, binaryPathName, executablePath string, expected conf.TunnelServiceLocator) error {
	if err := expected.Validate(); err != nil {
		return err
	}
	actual, recognized, err := wireHushLocatorFromServiceCommandLine(serviceName, binaryPathName, executablePath)
	if err != nil {
		return err
	}
	if !recognized {
		return fmt.Errorf("%w: service %q is not a WireHush TunnelID service", errWireHushTunnelServiceOwnershipConflict, serviceName)
	}
	if actual != expected {
		return fmt.Errorf("%w: service %q has locator %#v, expected %#v", errWireHushTunnelServiceOwnershipConflict, serviceName, actual, expected)
	}
	return nil
}

func verifyWireHushTunnelServiceOwnership(serviceName string, service *mgr.Service, executablePath string, expected conf.TunnelServiceLocator) error {
	config, err := service.Config()
	if err != nil {
		return err
	}
	return wireHushServiceCommandMatchesLocator(serviceName, config.BinaryPathName, executablePath, expected)
}
