/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package conf

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/sys/windows"
)

// ListTunnelRecords returns every validated record in one exact storage namespace.
func ListTunnelRecords(scope TunnelScope, ownerSID string) ([]TunnelRecord, error) {
	root, err := WireHushMachineDataRoot()
	if err != nil {
		return nil, err
	}
	return listTunnelRecordsAtRoot(root, scope, ownerSID)
}

func listTunnelRecordsAtRoot(root string, scope TunnelScope, ownerSID string) ([]TunnelRecord, error) {
	directory, err := tunnelRecordDirectoryFromRoot(root, scope, ownerSID)
	if err != nil {
		return nil, err
	}
	components := []string{root, filepath.Join(root, tunnelRecordConfigurationsDirectory)}
	if scope == TunnelScopePrivate {
		components = append(components, filepath.Join(root, tunnelRecordConfigurationsDirectory, tunnelRecordUsersDirectory), directory)
	} else {
		components = append(components, directory)
	}
	var namespace *os.File
	for index, component := range components {
		file, absent, err := openVerifiedExistingTunnelRecordDirectory(component)
		if err != nil {
			return nil, err
		}
		if absent {
			return []TunnelRecord{}, nil
		}
		if index == len(components)-1 {
			namespace = file
		} else if err := file.Close(); err != nil {
			return nil, err
		}
	}
	defer namespace.Close()
	entries, err := namespace.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	records := make([]TunnelRecord, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if isTunnelRecordTemporaryName(name) {
			info, err := entry.Info()
			if err != nil {
				return nil, err
			}
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("temporary record entry %q is not an ordinary file", name)
			}
			continue
		}
		id, err := tunnelRecordIDFromFileName(name)
		if err != nil {
			return nil, err
		}
		record, err := loadTunnelRecordAtRoot(root, scope, ownerSID, id)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].TunnelID.String() < records[j].TunnelID.String()
	})
	return records, nil
}

func openVerifiedExistingTunnelRecordDirectory(path string) (*os.File, bool, error) {
	path16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, false, err
	}
	handle, err := windows.CreateFile(path16, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_ATTRIBUTE_DIRECTORY, 0)
	if err == windows.ERROR_FILE_NOT_FOUND || err == windows.ERROR_PATH_NOT_FOUND {
		return nil, true, nil
	}
	if err != nil {
		return nil, false, err
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		windows.CloseHandle(handle)
		return nil, false, err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		windows.CloseHandle(handle)
		return nil, false, errors.New("tunnel record namespace component is actually a file")
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		windows.CloseHandle(handle)
		return nil, false, errors.New("tunnel record namespace component is a reparse point")
	}
	if err := verifyFinalPath(handle, path); err != nil {
		windows.CloseHandle(handle)
		return nil, false, err
	}
	file := os.NewFile(uintptr(handle), path)
	if file == nil {
		windows.CloseHandle(handle)
		return nil, false, errors.New("unable to create file from verified namespace handle")
	}
	return file, false, nil
}

func tunnelRecordIDFromFileName(name string) (TunnelID, error) {
	if !strings.HasSuffix(name, tunnelRecordFileSuffix) {
		return TunnelID{}, fmt.Errorf("unexpected tunnel record namespace entry %q", name)
	}
	id, err := ParseTunnelID(strings.TrimSuffix(name, tunnelRecordFileSuffix))
	if err != nil {
		return TunnelID{}, fmt.Errorf("invalid tunnel record filename %q: %w", name, err)
	}
	return id, nil
}

func isTunnelRecordTemporaryName(name string) bool {
	if len(name) != 64+len(".tmp") || !strings.HasSuffix(name, ".tmp") {
		return false
	}
	for _, character := range name[:64] {
		if !(character >= '0' && character <= '9') && !(character >= 'a' && character <= 'f') {
			return false
		}
	}
	return true
}
