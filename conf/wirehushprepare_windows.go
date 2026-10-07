/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */
package conf

import (
	"golang.org/x/sys/windows"
	"path/filepath"
)

// PrepareWireHushMachineData validates and protects the fixed machine hierarchy
// before any production manager/tunnel worker uses it. It accepts no client path.
func PrepareWireHushMachineData() (string, error) {
	root, err := WireHushMachineDataRoot()
	if err != nil {
		return "", err
	}
	sd, err := windows.SecurityDescriptorFromString(tunnelRecordDirectorySDDL)
	if err != nil {
		return "", err
	}
	for _, path := range []string{root, filepath.Join(root, "Settings"), filepath.Join(root, "Logs"), filepath.Join(root, "Migration")} {
		if err := ensureVerifiedTunnelRecordDirectory(path, sd); err != nil {
			return "", err
		}
	}
	return root, nil
}
