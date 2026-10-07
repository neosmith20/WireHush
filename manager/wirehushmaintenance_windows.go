/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */
package manager

import (
	"context"
	"errors"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
	"golang.zx2c4.com/wireguard/windows/conf"
	"golang.zx2c4.com/wireguard/windows/product"
	"path/filepath"
	"strings"
)

const wireHushMaintenanceEvent = `Global\WireHush.Installer.Maintenance.v1`

// The installer holds this kernel object only for its transaction lifetime.
// Power loss therefore cannot leave a persistent maintenance marker. A foreign
// owner or inaccessible object fails closed instead of permitting service start.
func wireHushMaintenanceActive(name string) (bool, error) {
	p, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return true, err
	}
	handle, err := windows.OpenEvent(windows.SYNCHRONIZE|windows.READ_CONTROL, false, p)
	if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
		return false, nil
	}
	if err != nil {
		return true, err
	}
	defer windows.CloseHandle(handle)
	sd, err := windows.GetSecurityInfo(handle, windows.SE_KERNEL_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return true, err
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return true, err
	}
	if owner.String() != "S-1-5-18" {
		return true, errWireHushAccessDenied
	}
	state, err := windows.WaitForSingleObject(handle, 0)
	if err != nil {
		return true, err
	}
	return state != uint32(windows.WAIT_TIMEOUT), nil
}

func requireWireHushSystem() error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	if user.User.Sid.String() != "S-1-5-18" {
		return errWireHushAccessDenied
	}
	return nil
}

// These commands are installer-only and have no RPC counterpart. Discovery and
// deletion retain the same executable/locator ownership checks as normal control.
func ShutdownOwnedV1Services(ctx context.Context) error {
	if err := requireWireHushSystem(); err != nil {
		return err
	}
	roots, err := conf.WireHushLegacyRoots()
	if err != nil {
		return err
	}
	executable := filepath.Join(filepath.Dir(roots["WireHush"]), "WireHush-Manager.exe")
	m, err := serviceManager()
	if err != nil {
		return err
	}
	names, err := m.ListServices()
	if err != nil {
		return err
	}
	for _, name := range names {
		if !strings.HasPrefix(name, product.TunnelServicePrefix) {
			continue
		}
		service, err := m.OpenService(name)
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			continue
		}
		if err != nil {
			return err
		}
		config, err := service.Config()
		if err != nil {
			service.Close()
			return err
		}
		_, recognized, err := wireHushLocatorFromServiceCommandLine(name, config.BinaryPathName, executable)
		if err != nil {
			service.Close()
			return err
		}
		if !recognized {
			service.Close()
			continue
		}
		if config.ServiceType != windows.SERVICE_WIN32_OWN_PROCESS || config.ServiceStartName != "LocalSystem" {
			service.Close()
			return errWireHushAccessDenied
		}
		err = removeWireHushTunnelAfterCleanup(ctx, service.Query, func() error { _, err := service.Control(svc.Stop); return err }, service.Delete)
		service.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func wireHushInstallerServiceOwned(name string, config mgr.Config, roots map[string]string) bool {
	if wireHushLegacyServiceOwned(name, config, roots) {
		return true
	}
	if config.ServiceType != windows.SERVICE_WIN32_OWN_PROCESS || config.ServiceStartName != "LocalSystem" {
		return false
	}
	executable := filepath.Join(filepath.Dir(roots["WireHush"]), "WireHush-Manager.exe")
	if name == product.ManagerServiceName {
		args, err := windows.DecomposeCommandLine(config.BinaryPathName)
		return err == nil && len(args) == 2 && strings.EqualFold(args[0], executable) && args[1] == "/managerservice"
	}
	_, recognized, err := wireHushLocatorFromServiceCommandLine(name, config.BinaryPathName, executable)
	return recognized && err == nil
}

// Run before standard MSI service controls or predecessor removal. A collision
// anywhere in the prefixes that predecessor cleanup touches aborts installation.
func PreflightV1Installer() error {
	if err := requireWireHushSystem(); err != nil {
		return err
	}
	roots, err := conf.WireHushLegacyRoots()
	if err != nil {
		return err
	}
	m, err := serviceManager()
	if err != nil {
		return err
	}
	names, err := m.ListServices()
	if err != nil {
		return err
	}
	for _, name := range names {
		if name != product.ManagerServiceName && name != product.LegacyManagerServiceName && !strings.HasPrefix(name, product.TunnelServicePrefix) && !strings.HasPrefix(name, product.LegacyTunnelServicePrefix) {
			continue
		}
		service, err := m.OpenService(name)
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			continue
		}
		if err != nil {
			return err
		}
		config, err := service.Config()
		service.Close()
		if err != nil {
			return err
		}
		if !wireHushInstallerServiceOwned(name, config, roots) {
			return errWireHushAccessDenied
		}
	}
	return nil
}

func FinalizeLegacyV1(ctx context.Context) error {
	if err := requireWireHushSystem(); err != nil {
		return err
	}
	return removeVerifiedWireHushLegacyServices(ctx)
}
