/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package conf

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"

	"golang.zx2c4.com/wireguard/windows/conf/dpapi"
)

const tunnelRecordDirectorySDDL = "O:SYG:SYD:PAI(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)"

// SaveTunnelRecord atomically saves an encrypted record at its ID-derived path.
func SaveTunnelRecord(record TunnelRecord, overwrite bool) error {
	if err := record.Validate(); err != nil {
		return err
	}
	root, err := WireHushMachineDataRoot()
	if err != nil {
		return err
	}
	return saveTunnelRecordAtRoot(root, record, overwrite, nil)
}

// LoadTunnelRecord loads the encrypted record identified by scope, owner, and ID.
func LoadTunnelRecord(scope TunnelScope, ownerSID string, id TunnelID) (TunnelRecord, error) {
	root, err := WireHushMachineDataRoot()
	if err != nil {
		return TunnelRecord{}, err
	}
	return loadTunnelRecordAtRoot(root, scope, ownerSID, id)
}

// DeleteTunnelRecord deletes the encrypted record identified by scope, owner, and ID.
func DeleteTunnelRecord(scope TunnelScope, ownerSID string, id TunnelID) error {
	root, err := WireHushMachineDataRoot()
	if err != nil {
		return err
	}
	return deleteTunnelRecordAtRoot(root, scope, ownerSID, id)
}

func tunnelRecordDPAPIDescription(id TunnelID) string {
	return "WireHush Tunnel " + id.String()
}

func saveTunnelRecordAtRoot(root string, record TunnelRecord, overwrite bool, directorySD *windows.SECURITY_DESCRIPTOR) error {
	if err := record.Validate(); err != nil {
		return err
	}
	if directorySD == nil {
		var err error
		directorySD, err = windows.SecurityDescriptorFromString(tunnelRecordDirectorySDDL)
		if err != nil {
			return err
		}
	}
	if _, err := ensureTunnelRecordDirectoryAtRoot(root, record.Scope, record.OwnerSID, directorySD); err != nil {
		return err
	}
	path, err := tunnelRecordPathFromRoot(root, record.Scope, record.OwnerSID, record.TunnelID)
	if err != nil {
		return err
	}
	plaintext, err := MarshalTunnelRecord(record)
	if err != nil {
		return err
	}
	ciphertext, err := dpapi.Encrypt(plaintext, tunnelRecordDPAPIDescription(record.TunnelID))
	if err != nil {
		return err
	}
	return writeLockedDownFile(path, overwrite, ciphertext)
}

func loadTunnelRecordAtRoot(root string, scope TunnelScope, ownerSID string, id TunnelID) (TunnelRecord, error) {
	path, err := tunnelRecordPathFromRoot(root, scope, ownerSID, id)
	if err != nil {
		return TunnelRecord{}, err
	}
	ciphertext, err := readVerifiedTunnelRecordFile(path)
	if err != nil {
		return TunnelRecord{}, err
	}
	plaintext, err := dpapi.Decrypt(ciphertext, tunnelRecordDPAPIDescription(id))
	if err != nil {
		return TunnelRecord{}, err
	}
	record, err := ParseTunnelRecord(plaintext)
	if err != nil {
		return TunnelRecord{}, err
	}
	if record.TunnelID != id || record.Scope != scope || record.OwnerSID != ownerSID {
		return TunnelRecord{}, errors.New("tunnel record metadata does not match its requested identity")
	}
	return record, nil
}

func deleteTunnelRecordAtRoot(root string, scope TunnelScope, ownerSID string, id TunnelID) error {
	path, err := tunnelRecordPathFromRoot(root, scope, ownerSID, id)
	if err != nil {
		return err
	}
	handle, err := openVerifiedTunnelRecordFile(path, windows.DELETE|windows.READ_CONTROL)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	yes := byte(1)
	return windows.SetFileInformationByHandle(handle, windows.FileDispositionInfo, &yes, 1)
}

func ensureTunnelRecordDirectoryAtRoot(root string, scope TunnelScope, ownerSID string, sd *windows.SECURITY_DESCRIPTOR) (string, error) {
	directory, err := tunnelRecordDirectoryFromRoot(root, scope, ownerSID)
	if err != nil {
		return "", err
	}
	components := []string{root, filepath.Join(root, tunnelRecordConfigurationsDirectory)}
	if scope == TunnelScopePrivate {
		components = append(components, filepath.Join(root, tunnelRecordConfigurationsDirectory, tunnelRecordUsersDirectory), directory)
	} else {
		components = append(components, directory)
	}
	for _, component := range components {
		if err := ensureVerifiedTunnelRecordDirectory(component, sd); err != nil {
			return "", err
		}
	}
	return directory, nil
}

func ensureVerifiedTunnelRecordDirectory(path string, sd *windows.SECURITY_DESCRIPTOR) error {
	path16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	sa := &windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	if err := windows.CreateDirectory(path16, sa); err != nil && err != windows.ERROR_ALREADY_EXISTS {
		return err
	}
	handle, err := windows.CreateFile(path16, windows.READ_CONTROL|windows.WRITE_OWNER|windows.WRITE_DAC, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_ATTRIBUTE_DIRECTORY, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		return errors.New("tunnel record directory is actually a file")
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return errors.New("tunnel record directory is a reparse point")
	}
	if err := verifyFinalPath(handle, path); err != nil {
		return err
	}
	return windows.SetKernelObjectSecurity(handle, windows.DACL_SECURITY_INFORMATION|windows.GROUP_SECURITY_INFORMATION|windows.OWNER_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, sd)
}

func readVerifiedTunnelRecordFile(path string) ([]byte, error) {
	handle, err := openVerifiedTunnelRecordFile(path, windows.GENERIC_READ)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(handle), path)
	if file == nil {
		windows.CloseHandle(handle)
		return nil, errors.New("unable to create file from verified record handle")
	}
	defer file.Close()
	return io.ReadAll(file)
}

func openVerifiedTunnelRecordFile(path string, access uint32) (windows.Handle, error) {
	path16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	handle, err := windows.CreateFile(path16, access, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return 0, err
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		windows.CloseHandle(handle)
		return 0, err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
		windows.CloseHandle(handle)
		return 0, errors.New("tunnel record path is a directory")
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		windows.CloseHandle(handle)
		return 0, errors.New("tunnel record path is a reparse point")
	}
	if err := verifyFinalPath(handle, path); err != nil {
		windows.CloseHandle(handle)
		return 0, err
	}
	return handle, nil
}

func verifyFinalPath(handle windows.Handle, expected string) error {
	expectedLong, err := longTunnelRecordPath(expected)
	if err != nil {
		return err
	}
	buffer := make([]uint16, windows.MAX_PATH+4)
	for {
		length, err := windows.GetFinalPathNameByHandle(handle, &buffer[0], uint32(len(buffer)), 0)
		if err != nil {
			return err
		}
		if length < uint32(len(buffer)) {
			if strings.EqualFold(`\\?\`+expectedLong, windows.UTF16ToString(buffer[:length])) {
				return nil
			}
			return fmt.Errorf("opened object path %q does not match expected path %q", windows.UTF16ToString(buffer[:length]), expected)
		}
		buffer = make([]uint16, length+1)
	}
}

func longTunnelRecordPath(path string) (string, error) {
	path16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	buffer := make([]uint16, windows.MAX_PATH+4)
	for {
		length, err := windows.GetLongPathName(path16, &buffer[0], uint32(len(buffer)))
		if err != nil {
			return "", err
		}
		if length < uint32(len(buffer)) {
			return windows.UTF16ToString(buffer[:length]), nil
		}
		buffer = make([]uint16, length+1)
	}
}
