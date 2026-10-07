/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */
package manager

import (
	"fmt"
	"golang.org/x/sys/windows"
	"testing"
	"time"
)

func TestWireHushMaintenanceForeignOwnerFailsClosed(t *testing.T) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	if user.User.Sid.String() == "S-1-5-18" {
		t.Skip("requires an ordinary test process")
	}
	name := fmt.Sprintf(`Local\WireHush.Maintenance.Test.%d`, time.Now().UnixNano())
	p, _ := windows.UTF16PtrFromString(name)
	handle, err := windows.CreateEvent(nil, 1, 0, p)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	active, err := wireHushMaintenanceActive(name)
	if !active || err == nil {
		t.Fatal("foreign unsignaled marker must fail closed")
	}
}
func TestWireHushMaintenanceAbsent(t *testing.T) {
	active, err := wireHushMaintenanceActive(fmt.Sprintf(`Local\WireHush.Absent.%d`, time.Now().UnixNano()))
	if err != nil || active {
		t.Fatalf("absent marker: %v %v", active, err)
	}
}
