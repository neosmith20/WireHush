/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package manager

import (
	"testing"

	"golang.org/x/sys/windows"
)

func TestLegacyTunnelServiceCommandLineRecognized(t *testing.T) {
	const executable = `C:\Program Files\WireHush\wirehush.exe`
	legacy := windows.ComposeCommandLine([]string{executable, "/tunnelservice", `C:\some path\name.conf`})
	recognized, err := legacyTunnelServiceCommandLineRecognized(legacy)
	if err != nil || !recognized {
		t.Fatalf("legacy recognition = %v, %v", recognized, err)
	}
	newService := windows.ComposeCommandLine([]string{executable, "/wirehushtunnelservice", "shared", "12345678-1234-4abc-8def-1234567890ab"})
	recognized, err = legacyTunnelServiceCommandLineRecognized(newService)
	if err != nil || recognized {
		t.Fatalf("new service recognition = %v, %v", recognized, err)
	}
}

func TestLegacyTunnelServiceCommandLineRecognizedRejectsCollisionAndMalformedInput(t *testing.T) {
	const executable = `C:\Program Files\WireHush\wirehush.exe`
	const collidingName = "1234567812344abc8def1234567890ab"
	command := windows.ComposeCommandLine([]string{executable, "/wirehushtunnelservice", "shared", "12345678-1234-4abc-8def-1234567890ab"})
	recognized, err := legacyTunnelServiceCommandLineRecognized(command)
	if err != nil || recognized {
		t.Fatalf("collision %s recognition = %v, %v", collidingName, recognized, err)
	}
	for _, command := range []string{
		windows.ComposeCommandLine([]string{executable, "/othercommand"}),
		windows.ComposeCommandLine([]string{executable}),
	} {
		recognized, err = legacyTunnelServiceCommandLineRecognized(command)
		if err != nil || recognized {
			t.Fatalf("command %q recognition = %v, %v", command, recognized, err)
		}
	}
	if recognized, err = legacyTunnelServiceCommandLineRecognized("C:\\Program Files\\WireHush\\wirehush.exe\x00 /tunnelservice"); err == nil || recognized {
		t.Fatalf("malformed recognition = %v, %v", recognized, err)
	}
}
