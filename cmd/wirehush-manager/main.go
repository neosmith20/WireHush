/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

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
	if len(args) == 1 && args[0] == "/installerpreflight" {
		return manager.PreflightV1Installer()
	}
	if len(args) == 1 && args[0] == "/migratelegacy" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		return manager.MigrateLegacyV1(ctx)
	}
	if len(args) == 1 && (args[0] == "/finalizelegacy" || args[0] == "/shutdownownedservices") {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if args[0] == "/finalizelegacy" {
			return manager.FinalizeLegacyV1(ctx)
		}
		return manager.ShutdownOwnedV1Services(ctx)
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
