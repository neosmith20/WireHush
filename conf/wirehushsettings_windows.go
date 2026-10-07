/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */
package conf

import (
	"errors"
	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/windows/bootstrap"
	"golang.zx2c4.com/wireguard/windows/conf/dpapi"
	"os"
	"path/filepath"
)

const wireHushBootstrapDescription = "WireHush V1 Bootstrap Settings"

func LoadWireHushBootstrapSettings() (bootstrap.Settings, error) {
	root, err := PrepareWireHushMachineData()
	if err != nil {
		return bootstrap.Settings{}, err
	}
	return loadWireHushBootstrapAtRoot(root)
}
func SaveWireHushBootstrapSettings(settings bootstrap.Settings) error {
	root, err := PrepareWireHushMachineData()
	if err != nil {
		return err
	}
	return saveWireHushBootstrapAtRoot(root, settings)
}
func loadWireHushBootstrapAtRoot(root string) (bootstrap.Settings, error) {
	data, err := readVerifiedTunnelRecordFile(filepath.Join(root, "Settings", "bootstrap-dns.dpapi"))
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
		return bootstrap.DefaultSettings(), nil
	}
	if err != nil {
		return bootstrap.Settings{}, err
	}
	plaintext, err := dpapi.Decrypt(data, wireHushBootstrapDescription)
	if err != nil {
		return bootstrap.Settings{}, err
	}
	return bootstrap.ParseSettings(plaintext)
}
func saveWireHushBootstrapAtRoot(root string, settings bootstrap.Settings) error {
	plaintext, err := bootstrap.MarshalSettings(settings)
	if err != nil {
		return err
	}
	ciphertext, err := dpapi.Encrypt(plaintext, wireHushBootstrapDescription)
	if err != nil {
		return err
	}
	return writeLockedDownFile(filepath.Join(root, "Settings", "bootstrap-dns.dpapi"), true, ciphertext)
}
