/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2019-2026 WireGuard LLC. All Rights Reserved.
 */

package tunnel

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/netip"
	"os"
	"runtime"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
	"golang.zx2c4.com/wireguard/windows/bootstrap"
	"golang.zx2c4.com/wireguard/windows/conf"
	"golang.zx2c4.com/wireguard/windows/dohruntime"
	"golang.zx2c4.com/wireguard/windows/driver"
	"golang.zx2c4.com/wireguard/windows/elevate"
	"golang.zx2c4.com/wireguard/windows/ringlogger"
	"golang.zx2c4.com/wireguard/windows/services"
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
)

type tunnelService struct {
	Path          string
	RecordLocator *conf.TunnelServiceLocator
}

func (service *tunnelService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
	serviceState := svc.StartPending
	changes <- svc.Status{State: serviceState}

	var watcher *interfaceWatcher
	var adapter *driver.Adapter
	var luid winipcfg.LUID
	var config *conf.Config
	var runtimeInterfaceName string
	var adapterGUID *windows.GUID
	var encryptedDNSSession *dohruntime.Session
	var configuredBootstrap []netip.Addr
	var err error
	serviceError := services.ErrorSuccess

	defer func() {
		svcSpecificEC, exitCode = services.DetermineErrorCode(err, serviceError)
		logErr := services.CombineErrors(err, serviceError)
		if logErr != nil {
			log.Println(logErr)
		}
		serviceState = svc.StopPending
		changes <- svc.Status{State: serviceState}

		stopIt := make(chan bool, 1)
		go func() {
			t := time.NewTicker(time.Second * 30)
			for {
				select {
				case <-t.C:
					t.Stop()
					buf := make([]byte, 1024)
					for {
						n := runtime.Stack(buf, true)
						if n < len(buf) {
							buf = buf[:n]
							break
						}
						buf = make([]byte, 2*len(buf))
					}
					lines := bytes.Split(buf, []byte{'\n'})
					log.Println("Failed to shutdown after 30 seconds. Probably dead locked. Printing stack and killing.")
					for _, line := range lines {
						if len(bytes.TrimSpace(line)) > 0 {
							log.Println(string(line))
						}
					}
					os.Exit(777)
					return
				case <-stopIt:
					t.Stop()
					return
				}
			}
		}()

		if encryptedDNSSession != nil {
			cleanupErr := encryptedDNSSession.Close()
			if service.RecordLocator != nil {
				for retry := 0; cleanupErr != nil && retry < 2; retry++ {
					time.Sleep(100 * time.Millisecond)
					cleanupErr = encryptedDNSSession.Close()
				}
			}
			if cleanupErr != nil && logErr == nil {
				logErr = fmt.Errorf("unable to restore encrypted DNS state: %w", cleanupErr)
			}
		}
		if logErr == nil && adapter != nil && config != nil {
			logErr = runScriptCommand(config.Interface.PreDown, runtimeInterfaceName)
		}
		if watcher != nil {
			watcher.Destroy()
		}
		if adapter != nil {
			adapter.Close()
		}
		if logErr == nil && adapter != nil && config != nil {
			postDownErr := runScriptCommand(config.Interface.PostDown, runtimeInterfaceName)
			if service.RecordLocator != nil {
				logErr = postDownErr
			}
		}
		if service.RecordLocator != nil && logErr != nil {
			// Cleanup happens after the initial error-code calculation. Never
			// report successful exit when DNS restoration or a down hook failed.
			svcSpecificEC, exitCode = services.DetermineErrorCode(logErr, services.ErrorSetNetConfig)
		}
		if service.RecordLocator != nil && logErr != nil && adapter == nil && encryptedDNSSession == nil {
			// Endpoint resolution and source loading precede adapter creation and
			// every network mutation. Preserve failure evidence while allowing a
			// corrected configuration to retry without administrator repair.
			svcSpecificEC, exitCode = true, uint32(services.ErrorStartupBeforeNetwork)
		}
		stopIt <- true
		log.Println("Shutting down")
	}()

	var logFile string
	if service.RecordLocator != nil {
		var protectedLog *os.File
		protectedLog, err = conf.OpenWireHushLogFile()
		if err == nil {
			err = ringlogger.InitWireHushLogger(protectedLog, "TUN")
		}
	} else {
		logFile, err = conf.LogFile(true)
		if err == nil {
			err = ringlogger.InitGlobalLogger(logFile, "TUN")
		}
	}
	if err != nil {
		serviceError = services.ErrorRingloggerOpen
		return
	}

	config, runtimeInterfaceName, adapterGUID, err = service.prepareSource()
	if err != nil {
		serviceError = services.ErrorLoadConfiguration
		return
	}
	config.DeduplicateNetworkEntries()

	log.SetPrefix(fmt.Sprintf("[%s] ", config.Name))

	services.PrintStarting()

	if services.StartedAtBoot() {
		if m, err := mgr.Connect(); err == nil {
			if lockStatus, err := m.LockStatus(); err == nil && lockStatus.IsLocked {
				/* If we don't do this, then the driver installation will block forever, because
				 * installing a network adapter starts the driver service too. Apparently at boot time,
				 * Windows 8.1 locks the SCM for each service start, creating a deadlock if we don't
				 * announce that we're running before starting additional services.
				 */
				log.Printf("SCM locked for %v by %s, marking service as started", lockStatus.Age, lockStatus.Owner)
				serviceState = svc.Running
				changes <- svc.Status{State: serviceState}
			}
			m.Disconnect()
		}
	}

	evaluateStaticPitfalls()

	log.Println("Watching network interfaces")
	watcher, err = watchInterface()
	if err != nil {
		serviceError = services.ErrorSetNetConfig
		return
	}

	log.Println("Resolving DNS names")
	if encryptedDNSConfigured(config) {
		if service.RecordLocator != nil {
			settings, loadErr := conf.LoadWireHushBootstrapSettings()
			err = loadErr
			if err == nil {
				configuredBootstrap, err = settings.EnabledResolvers()
			}
		} else {
			configuredBootstrap, err = configuredBootstrapResolvers()
		}
		if err != nil {
			serviceError = services.ErrorSetNetConfig
			return
		}
		configuredBootstrap = bootstrapResolversForConfig(config, configuredBootstrap)
		if len(configuredBootstrap) == 0 {
			serviceError = services.ErrorSetNetConfig
			return
		}
		resolver := bootstrap.NewResolver(configuredBootstrap)
		resolver.Timeout = dohRuntimeTimeout
		err = config.ResolveEndpointsWith(func(host string) ([]netip.Addr, error) {
			return resolver.ResolveHost(context.Background(), host)
		})
	} else {
		err = config.ResolveEndpoints()
	}
	if err != nil {
		serviceError = services.ErrorDNSLookup
		return
	}

	log.Println("Creating network adapter")
	for i := range 15 {
		if i > 0 {
			time.Sleep(time.Second)
			log.Printf("Retrying adapter creation after failure because system just booted (T+%v): %v", windows.DurationSinceBoot(), err)
		}
		adapter, err = driver.CreateAdapter(runtimeInterfaceName, "WireGuard", adapterGUID)
		if err == nil || !services.StartedAtBoot() {
			break
		}
	}
	if err != nil {
		err = fmt.Errorf("Error creating adapter: %w", err)
		serviceError = services.ErrorCreateNetworkAdapter
		return
	}
	luid = adapter.LUID()
	driverVersion, err := driver.RunningVersion()
	if err != nil {
		log.Printf("Warning: unable to determine driver version: %v", err)
	} else {
		log.Printf("Using WireGuardNT/%d.%d", (driverVersion>>16)&0xffff, driverVersion&0xffff)
	}
	err = adapter.SetLogging(driver.AdapterLogOn)
	if err != nil {
		err = fmt.Errorf("Error enabling adapter logging: %w", err)
		serviceError = services.ErrorCreateNetworkAdapter
		return
	}

	err = runScriptCommand(config.Interface.PreUp, runtimeInterfaceName)
	if err != nil {
		serviceError = services.ErrorRunScript
		return
	}

	err = enableFirewall(config, luid, encryptedDNSConfigured(config), configuredBootstrap)
	if err != nil {
		serviceError = services.ErrorFirewall
		return
	}

	log.Println("Dropping privileges")
	err = elevate.DropAllPrivileges(true)
	if err != nil {
		serviceError = services.ErrorDropPrivileges
		return
	}

	log.Println("Setting interface configuration")
	err = adapter.SetConfiguration(config.ToDriverConfiguration())
	if err != nil {
		serviceError = services.ErrorDeviceSetConfig
		return
	}
	err = adapter.SetAdapterState(driver.AdapterStateUp)
	if err != nil {
		serviceError = services.ErrorDeviceBringUp
		return
	}
	watcher.Configure(adapter, config, luid)
	if encryptedDNSConfigured(config) {
		log.Println("Waiting for initial interface configuration before encrypted DNS startup")
		serviceError, err = watcher.WaitForInitialConfiguration(configuredInterfaceFamilies(config))
		if err != nil {
			return
		}
	}

	log.Println("Starting encrypted DNS runtime")
	encryptedDNSSession, err = activateEncryptedDNS(context.Background(), config, luid, adapter, configuredBootstrap)
	if err != nil {
		serviceError = services.ErrorSetNetConfig
		return
	}
	if encryptedDNSSession != nil {
		watcher.SetRecovery(func() error {
			return encryptedDNSSession.Recover(context.Background())
		})
	}

	err = runScriptCommand(config.Interface.PostUp, runtimeInterfaceName)
	if err != nil {
		serviceError = services.ErrorRunScript
		return
	}

	started := encryptedDNSConfigured(config)
	if started {
		serviceState = svc.Running
		changes <- svc.Status{State: serviceState, Accepts: svc.AcceptStop | svc.AcceptShutdown}
		log.Println("Startup complete")
	} else {
		changes <- svc.Status{State: serviceState, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	}
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Stop, svc.Shutdown:
				return
			case svc.Interrogate:
				changes <- c.CurrentStatus
			default:
				log.Printf("Unexpected service control request #%d\n", c)
			}
		case <-watcher.started:
			if !started {
				serviceState = svc.Running
				changes <- svc.Status{State: serviceState, Accepts: svc.AcceptStop | svc.AcceptShutdown}
				log.Println("Startup complete")
				started = true
			}
		case e := <-watcher.errors:
			serviceError, err = e.serviceError, e.err
			return
		}
	}
}

func Run(confPath string) error {
	name, err := conf.NameFromPath(confPath)
	if err != nil {
		return err
	}
	serviceName, err := conf.ServiceNameOfTunnel(name)
	if err != nil {
		return err
	}
	return svc.Run(serviceName, &tunnelService{Path: confPath})
}
