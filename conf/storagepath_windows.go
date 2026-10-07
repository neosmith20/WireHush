/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package conf

import (
	"errors"
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows"

	"golang.zx2c4.com/wireguard/windows/product"
)

const (
	tunnelRecordConfigurationsDirectory = "Configurations"
	tunnelRecordUsersDirectory          = "Users"
	tunnelRecordSharedDirectory         = "Shared"
	tunnelRecordFileSuffix              = ".conf.dpapi"
)

// WireHushMachineDataRoot returns the future WireHush machine-data root.
// It derives a path only; it does not establish filesystem trust or create it.
func WireHushMachineDataRoot() (string, error) {
	programData, err := windows.KnownFolderPath(windows.FOLDERID_ProgramData, windows.KF_FLAG_DEFAULT)
	if err != nil {
		return "", err
	}
	return filepath.Join(programData, product.Name), nil
}

// TunnelRecordDirectory returns the future scoped record directory path.
// It derives a path only; future storage code must safely create and open it.
func TunnelRecordDirectory(scope TunnelScope, ownerSID string) (string, error) {
	root, err := WireHushMachineDataRoot()
	if err != nil {
		return "", err
	}
	return tunnelRecordDirectoryFromRoot(root, scope, ownerSID)
}

func tunnelRecordDirectoryFromRoot(root string, scope TunnelScope, ownerSID string) (string, error) {
	switch scope {
	case TunnelScopePrivate:
		if err := validateCanonicalOwnerSID(ownerSID); err != nil {
			return "", err
		}
		return filepath.Join(root, tunnelRecordConfigurationsDirectory, tunnelRecordUsersDirectory, ownerSID), nil
	case TunnelScopeShared:
		if ownerSID != "" {
			return "", errors.New("shared tunnel owner SID must be empty")
		}
		return filepath.Join(root, tunnelRecordConfigurationsDirectory, tunnelRecordSharedDirectory), nil
	default:
		return "", errors.New("tunnel scope is not valid")
	}
}

// TunnelRecordPath returns the future scoped record file path.
// It derives a path only; it does not create or open a filesystem object.
func TunnelRecordPath(scope TunnelScope, ownerSID string, id TunnelID) (string, error) {
	root, err := WireHushMachineDataRoot()
	if err != nil {
		return "", err
	}
	return tunnelRecordPathFromRoot(root, scope, ownerSID, id)
}

func tunnelRecordPathFromRoot(root string, scope TunnelScope, ownerSID string, id TunnelID) (string, error) {
	if !id.Valid() {
		return "", errors.New("TunnelID is not valid")
	}
	directory, err := tunnelRecordDirectoryFromRoot(root, scope, ownerSID)
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, id.String()+tunnelRecordFileSuffix), nil
}

func validateCanonicalOwnerSID(ownerSID string) error {
	if ownerSID == "" {
		return errors.New("private tunnel owner SID is required")
	}
	sid, err := windows.StringToSid(ownerSID)
	if err != nil {
		return fmt.Errorf("private tunnel owner SID is not valid: %w", err)
	}
	if sid.String() != ownerSID {
		return errors.New("private tunnel owner SID is not canonical")
	}
	return nil
}
