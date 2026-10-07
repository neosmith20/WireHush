/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */
package manager

import (
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
	"testing"
)

func TestWireHushInstallerRejectsServiceCollisions(t *testing.T) {
	roots := map[string]string{"WireHush": `C:\Program Files\WireHush\Data`, "TunnelMint": `C:\Program Files\TunnelMint\Data`}
	valid := mgr.Config{ServiceType: windows.SERVICE_WIN32_OWN_PROCESS, ServiceStartName: "LocalSystem", BinaryPathName: `"C:\Program Files\WireHush\WireHush-Manager.exe" /managerservice`}
	if !wireHushInstallerServiceOwned("WireHushManager", valid, roots) {
		t.Fatal("canonical manager rejected")
	}
	for _, command := range []string{`"C:\Other\WireHush-Manager.exe" /managerservice`, `"C:\Program Files\WireHush\WireHush-Manager.exe" /managerservice extra`, `"C:\Program Files\WireHush\WireHush-Manager.exe" /update`} {
		bad := valid
		bad.BinaryPathName = command
		if wireHushInstallerServiceOwned("WireHushManager", bad, roots) {
			t.Fatal("foreign manager accepted")
		}
	}
	bad := valid
	bad.ServiceStartName = "OtherAccount"
	if wireHushInstallerServiceOwned("WireHushManager", bad, roots) {
		t.Fatal("foreign account accepted")
	}
	if wireHushInstallerServiceOwned("WireGuardManager", valid, roots) {
		t.Fatal("upstream service accepted")
	}
}
func TestWireHushInstallerCommandsRequireSystem(t *testing.T) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	if user.User.Sid.String() == "S-1-5-18" {
		t.Skip("requires non-System test runner")
	}
	if err := PreflightV1Installer(); err == nil {
		t.Fatal("installer control exposed to non-System caller")
	}
}
