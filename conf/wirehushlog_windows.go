/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */
package conf

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"unsafe"
)

// Open a verified protected handle before the ring buffer can truncate or map
// it. No path supplied by an RPC client participates in log storage.
func OpenWireHushLogFile() (*os.File, error) {
	root, err := PrepareWireHushMachineData()
	if err != nil {
		return nil, err
	}
	return openWireHushLogFileAtRoot(root, nil)
}
func openWireHushLogFileAtRoot(root string, sd *windows.SECURITY_DESCRIPTOR) (*os.File, error) {
	if sd == nil {
		var err error
		sd, err = windows.SecurityDescriptorFromString("O:SYG:SYD:P(A;;FA;;;SY)(A;;SD;;;BA)")
		if err != nil {
			return nil, err
		}
	}
	path := filepath.Join(root, "Logs", "backend.bin")
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	sa := &windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE|windows.WRITE_OWNER|windows.WRITE_DAC, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, sa, windows.OPEN_ALWAYS, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*os.File, error) { windows.CloseHandle(handle); return nil, err }
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return fail(err)
	}
	if info.FileAttributes&(windows.FILE_ATTRIBUTE_REPARSE_POINT|windows.FILE_ATTRIBUTE_DIRECTORY) != 0 {
		return fail(errors.New("unsafe log file object"))
	}
	if info.NumberOfLinks != 1 {
		return fail(errors.New("hard-linked log file is not trusted"))
	}
	if err := verifyFinalPath(handle, path); err != nil {
		return fail(err)
	}
	if err := windows.SetKernelObjectSecurity(handle, windows.DACL_SECURITY_INFORMATION|windows.GROUP_SECURITY_INFORMATION|windows.OWNER_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, sd); err != nil {
		return fail(err)
	}
	return os.NewFile(uintptr(handle), path), nil
}
