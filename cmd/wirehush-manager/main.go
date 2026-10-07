/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */
package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"golang.zx2c4.com/wireguard/windows/conf"
	"golang.zx2c4.com/wireguard/windows/manager"
	"golang.zx2c4.com/wireguard/windows/tunnel"
	"golang.zx2c4.com/wireguard/windows/version"
)

func run(args []string) error {
	if len(args) == 1 && args[0] == "/version" {
		fmt.Println("WireHush-Manager " + version.Number)
		return nil
	}
	if len(args) == 1 && args[0] == "/managerservice" {
		return manager.RunV1()
	}
	if len(args) == 1 && args[0] == "/installmanagerservice" {
		return manager.InstallManager()
	}
	if len(args) == 1 && args[0] == "/uninstallmanagerservice" {
		return manager.UninstallManager()
	}
	if len(args) > 0 && args[0] == conf.WireHushTunnelServiceCommand {
		locator, err := conf.ParseWireHushTunnelServiceArgs(args)
		if err != nil {
			return err
		}
		root, err := conf.PrepareWireHushMachineData()
		if err != nil {
			return err
		}
		conf.PresetRootDirectory(root)
		return tunnel.RunWireHush(locator)
	}
	return fmt.Errorf("unsupported backend command")
}
func main() {
	// No raw parser errors, command-line arguments, keys, or endpoints are
	// emitted to an unprotected standard stream. Protected logging is separate.
	log.SetOutput(io.Discard)
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "WireHush backend could not complete the requested operation")
		os.Exit(1)
	}
}
