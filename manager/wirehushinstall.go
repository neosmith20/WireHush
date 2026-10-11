/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2019-2026 WireGuard LLC. All Rights Reserved.
 */

package manager

import (
	"context"
	"errors"
	"os"
	"sync"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"golang.zx2c4.com/wireguard/windows/conf"
	"golang.zx2c4.com/wireguard/windows/product"
)

var cachedServiceManager *mgr.Mgr
var cachedServiceManagerLock sync.Mutex

func serviceManager() (*mgr.Mgr, error) {
	cachedServiceManagerLock.Lock()
	defer cachedServiceManagerLock.Unlock()
	if cachedServiceManager != nil {
		return cachedServiceManager, nil
	}
	m, err := mgr.Connect()
	if err != nil {
		return nil, err
	}
	cachedServiceManager = m
	return cachedServiceManager, nil
}

func managerServiceConfig() mgr.Config {
	return mgr.Config{
		ServiceType:  windows.SERVICE_WIN32_OWN_PROCESS,
		StartType:    mgr.StartManual,
		ErrorControl: mgr.ErrorNormal,
		DisplayName:  product.ManagerServiceDisplayName,
	}
}

func wireHushTunnelServiceIdentity(locator conf.TunnelServiceLocator) (string, []string, error) {
	if err := locator.Validate(); err != nil {
		return "", nil, err
	}
	serviceName, err := conf.ServiceNameOfTunnelID(locator.TunnelID)
	if err != nil {
		return "", nil, err
	}
	args, err := conf.WireHushTunnelServiceArgs(locator)
	if err != nil {
		return "", nil, err
	}
	return serviceName, args, nil
}

func wireHushTunnelServiceConfig(record conf.TunnelRecord) mgr.Config {
	return mgr.Config{
		ServiceType:  windows.SERVICE_WIN32_OWN_PROCESS,
		StartType:    mgr.StartManual,
		ErrorControl: mgr.ErrorNormal,
		Dependencies: []string{"Nsi", "TcpIp"},
		DisplayName:  product.TunnelServiceDisplayPrefix + record.TunnelID.Token(),
		SidType:      windows.SERVICE_SID_TYPE_UNRESTRICTED,
	}
}

func waitForWireHushTunnelRemoval(m *mgr.Mgr, serviceName, executablePath string, locator conf.TunnelServiceLocator) error {
	ctx, cancel := context.WithTimeout(context.Background(), wireHushOperationTimeout)
	defer cancel()
	return waitForWireHushTunnelRemovalContext(ctx, m, serviceName, executablePath, locator)
}

func waitForWireHushTunnelRemovalContext(ctx context.Context, m *mgr.Mgr, serviceName, executablePath string, locator conf.TunnelServiceLocator) error {
	return pollWireHushTunnelRemoval(ctx, func() error {
		service, err := m.OpenService(serviceName)
		if err == nil {
			err = verifyWireHushTunnelServiceOwnership(serviceName, service, executablePath, locator)
			service.Close()
		}
		return err
	})
}

func InstallWireHushTunnel(locator conf.TunnelServiceLocator) error {
	ctx, cancel := context.WithTimeout(context.Background(), wireHushOperationTimeout)
	defer cancel()
	return InstallWireHushTunnelContext(ctx, locator)
}

func InstallWireHushTunnelContext(ctx context.Context, locator conf.TunnelServiceLocator) error {
	ctx, cancel := context.WithTimeout(ctx, wireHushOperationTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	serviceName, args, err := wireHushTunnelServiceIdentity(locator)
	if err != nil {
		return err
	}
	record, err := conf.LoadTunnelRecord(locator.Scope, locator.OwnerSID, locator.TunnelID)
	if err != nil {
		return err
	}
	m, err := serviceManager()
	if err != nil {
		return err
	}
	path, err := os.Executable()
	if err != nil {
		return err
	}
	service, err := m.OpenService(serviceName)
	if err == nil {
		err = verifyWireHushTunnelServiceOwnership(serviceName, service, path, locator)
		if err != nil {
			service.Close()
			if err != windows.ERROR_SERVICE_MARKED_FOR_DELETE {
				return err
			}
			if err := waitForWireHushTunnelRemovalContext(ctx, m, serviceName, path, locator); err != nil {
				return err
			}
		} else {
			status, statusErr := service.Query()
			if statusErr != nil && statusErr != windows.ERROR_SERVICE_MARKED_FOR_DELETE {
				service.Close()
				return statusErr
			}
			if statusErr == windows.ERROR_SERVICE_MARKED_FOR_DELETE {
				service.Close()
				if err := waitForWireHushTunnelRemovalContext(ctx, m, serviceName, path, locator); err != nil {
					return err
				}
			} else {
				if statusErr == nil && status.State == svc.Stopped && wireHushTunnelNeedsRepair(status) {
					service.Close()
					return errWireHushCleanupFailed
				}
				if statusErr == nil && status.State != svc.Stopped {
					service.Close()
					return errors.New("WireHush tunnel already installed and running")
				}
				deleteErr := service.Delete()
				service.Close()
				if deleteErr != nil && deleteErr != windows.ERROR_SERVICE_MARKED_FOR_DELETE {
					return deleteErr
				}
				if err := waitForWireHushTunnelRemovalContext(ctx, m, serviceName, path, locator); err != nil {
					return err
				}
			}
		}
	} else if err == windows.ERROR_SERVICE_MARKED_FOR_DELETE {
		if err := waitForWireHushTunnelRemovalContext(ctx, m, serviceName, path, locator); err != nil {
			return err
		}
	} else if err != windows.ERROR_SERVICE_DOES_NOT_EXIST {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	service, err = m.CreateService(serviceName, path, wireHushTunnelServiceConfig(record), "/wirehush-service-provisioning")
	if err != nil {
		return err
	}
	if err := finishWireHushWorkerProvision(func() error { return protectWireHushWorkerService(service) }, func() error {
		config := wireHushTunnelServiceConfig(record)
		config.BinaryPathName = windows.ComposeCommandLine(append([]string{path}, args...))
		return service.UpdateConfig(config)
	}, func() error { return service.Start() }); err != nil {
		// A failed protection step leaves no private locator in the placeholder.
		// Remove only that harmless binding; preserve canonical workers because
		// a Start error does not prove that networking was never initialized.
		if current, queryErr := service.Config(); queryErr == nil {
			command, parseErr := windows.DecomposeCommandLine(current.BinaryPathName)
			if parseErr == nil && len(command) == 2 && command[0] == path && command[1] == "/wirehush-service-provisioning" {
				_ = service.Delete()
			}
		}
		service.Close()
		return err
	}
	go trackWireHushTunnelService(locator, service)
	return nil
}

func UninstallWireHushTunnel(locator conf.TunnelServiceLocator) error {
	ctx, cancel := context.WithTimeout(context.Background(), wireHushOperationTimeout)
	defer cancel()
	return UninstallWireHushTunnelContext(ctx, locator)
}

func UninstallWireHushTunnelContext(ctx context.Context, locator conf.TunnelServiceLocator) error {
	serviceName, _, err := wireHushTunnelServiceIdentity(locator)
	if err != nil {
		return err
	}
	m, err := serviceManager()
	if err != nil {
		return err
	}
	path, err := os.Executable()
	if err != nil {
		return err
	}
	service, err := m.OpenService(serviceName)
	if err != nil {
		return err
	}
	err = verifyWireHushTunnelServiceOwnership(serviceName, service, path, locator)
	if err != nil {
		service.Close()
		if err == windows.ERROR_SERVICE_MARKED_FOR_DELETE {
			return nil
		}
		return err
	}
	defer service.Close()
	return removeWireHushTunnelAfterCleanup(ctx, service.Query, func() error { _, err := service.Control(svc.Stop); return err }, service.Delete)
}

func wireHushTunnelUninstallResult(stopErr, deleteErr error) error {
	if deleteErr != nil && deleteErr != windows.ERROR_SERVICE_MARKED_FOR_DELETE {
		return deleteErr
	}
	if stopErr != nil && stopErr != windows.ERROR_SERVICE_NOT_ACTIVE && stopErr != windows.ERROR_SERVICE_MARKED_FOR_DELETE {
		return stopErr
	}
	return nil
}

func WaitForWireHushTunnelStop(locator conf.TunnelServiceLocator) error {
	ctx, cancel := context.WithTimeout(context.Background(), wireHushOperationTimeout)
	defer cancel()
	return WaitForWireHushTunnelStopContext(ctx, locator)
}

// WaitForWireHushTunnelStopContext waits for verified service removal without
// allowing a caller deadline to become an unbounded cleanup wait.
func WaitForWireHushTunnelStopContext(ctx context.Context, locator conf.TunnelServiceLocator) error {
	serviceName, _, err := wireHushTunnelServiceIdentity(locator)
	if err != nil {
		return err
	}
	m, err := serviceManager()
	if err != nil {
		return err
	}
	path, err := os.Executable()
	if err != nil {
		return err
	}
	return waitForWireHushTunnelRemovalContext(ctx, m, serviceName, path, locator)
}

func WireHushTunnelState(locator conf.TunnelServiceLocator) (TunnelState, error) {
	serviceName, _, err := wireHushTunnelServiceIdentity(locator)
	if err != nil {
		return TunnelUnknown, err
	}
	m, err := serviceManager()
	if err != nil {
		return TunnelUnknown, err
	}
	service, err := m.OpenService(serviceName)
	if err != nil {
		switch err {
		case windows.ERROR_SERVICE_DOES_NOT_EXIST:
			return TunnelStopped, nil
		case windows.ERROR_SERVICE_MARKED_FOR_DELETE:
			return TunnelStopping, nil
		default:
			return TunnelUnknown, err
		}
	}
	defer service.Close()
	path, err := os.Executable()
	if err != nil {
		return TunnelUnknown, err
	}
	err = verifyWireHushTunnelServiceOwnership(serviceName, service, path, locator)
	if err != nil {
		if err == windows.ERROR_SERVICE_MARKED_FOR_DELETE {
			return TunnelStopping, nil
		}
		return TunnelUnknown, err
	}
	status, err := service.Query()
	if err != nil {
		if err == windows.ERROR_SERVICE_MARKED_FOR_DELETE {
			return TunnelStopping, nil
		}
		return TunnelUnknown, err
	}
	return wireHushTunnelStateFromServiceStatus(status), nil
}

func wireHushTunnelStateFromServiceStatus(status svc.Status) TunnelState {
	switch status.State {
	case svc.Stopped:
		if wireHushTunnelNeedsRepair(status) {
			return TunnelUnknown
		}
		return TunnelStopped
	case svc.StopPending:
		return TunnelStopping
	case svc.Running:
		return TunnelStarted
	case svc.StartPending:
		return TunnelStarting
	default:
		return TunnelUnknown
	}
}
