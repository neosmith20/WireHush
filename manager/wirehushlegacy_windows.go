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
	"time"
)

func wireHushLegacyServiceOwned(name string, config mgr.Config, roots map[string]string) bool {
	args, err := windows.DecomposeCommandLine(config.BinaryPathName)
	if err != nil || len(args) < 2 || config.ServiceType != windows.SERVICE_WIN32_OWN_PROCESS || config.ServiceStartName != "LocalSystem" {
		return false
	}
	for _, root := range roots {
		ownedExecutable := false
		for _, binary := range []string{"wirehush.exe", "tunnelmint.exe"} {
			if strings.EqualFold(args[0], filepath.Join(filepath.Dir(root), binary)) {
				ownedExecutable = true
			}
		}
		if !ownedExecutable {
			continue
		}
		if (name == product.LegacyManagerServiceName || name == product.ManagerServiceName) && len(args) == 2 && args[1] == "/managerservice" {
			return true
		}
		if len(args) != 3 || args[1] != "/tunnelservice" || !strings.EqualFold(filepath.Dir(args[2]), filepath.Join(root, "Configurations")) {
			continue
		}
		tunnel, err := conf.NameFromPath(args[2])
		if err != nil {
			continue
		}
		if name == product.LegacyTunnelServicePrefix+tunnel || name == product.TunnelServicePrefix+tunnel {
			return true
		}
	}
	return false
}
func verifiedWireHushLegacyServices(action func(string, *mgr.Service) error) error {
	roots, err := conf.WireHushLegacyRoots()
	if err != nil {
		return err
	}
	manager, err := serviceManager()
	if err != nil {
		return err
	}
	names, err := manager.ListServices()
	if err != nil {
		return err
	}
	for _, name := range names {
		if name != product.LegacyManagerServiceName && name != product.ManagerServiceName && !strings.HasPrefix(name, product.LegacyTunnelServicePrefix) && !strings.HasPrefix(name, product.TunnelServicePrefix) {
			continue
		}
		service, err := manager.OpenService(name)
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
		owned := wireHushLegacyServiceOwned(name, config, roots)
		if !owned {
			service.Close()
			continue
		} // Never act on an unverified service collision.
		err = action(name, service)
		service.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
func stopVerifiedWireHushLegacyServices(ctx context.Context) error {
	return verifiedWireHushLegacyServices(func(_ string, service *mgr.Service) error {
		current, err := service.Query()
		if err != nil {
			return err
		}
		if current.State != svc.Stopped && current.State != svc.StopPending {
			if _, err := service.Control(svc.Stop); err != nil && !errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
				return err
			}
		}
		for {
			if err := ctx.Err(); err != nil {
				return err
			}
			current, err = service.Query()
			if err != nil {
				return err
			}
			if current.State == svc.Stopped {
				return wireHushTunnelServiceExitError(current)
			}
			timer := time.NewTimer(50 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	})
}
func removeVerifiedWireHushLegacyServices(ctx context.Context) error {
	return verifiedWireHushLegacyServices(func(_ string, service *mgr.Service) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		status, err := service.Query()
		if err != nil {
			return err
		}
		if status.State != svc.Stopped || wireHushTunnelServiceExitError(status) != nil {
			return errWireHushCleanupFailed
		}
		return service.Delete()
	})
}
