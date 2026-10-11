/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package tunnel

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
)

const wireHushConnectionName = "WireHush Connection"
const wireHushTunnelType = "WireHush"

// Use the same Windows connection-name API as WireGuardNT, but deliberately
// do not use its collision fallback, which can rename a foreign adapter.
// The GUID comes from the adapter we just created, never an alias search.
var nciSetConnectionName = windows.NewLazySystemDLL("nci.dll").NewProc("NciSetConnectionName")

func setWireHushConnectionName(luid winipcfg.LUID) error {
	guid, err := luid.GUID()
	if err != nil {
		return err
	}
	name, err := windows.UTF16PtrFromString(wireHushConnectionName)
	if err != nil {
		return err
	}
	if err := nciSetConnectionName.Find(); err != nil {
		return err
	}
	result, _, _ := syscall.SyscallN(nciSetConnectionName.Addr(), uintptr(unsafe.Pointer(guid)), uintptr(unsafe.Pointer(name)))
	if result != 0 {
		return syscall.Errno(result)
	}
	return nil
}

// Branding is cosmetic: a collision or unavailable naming API must not break
// networking, change private adapter identity, or mutate another connection.
func wireHushConnectionAlias(runtimeName string, rename func() error) (string, error) {
	if err := rename(); err != nil {
		return runtimeName, err
	}
	return wireHushConnectionName, nil
}
