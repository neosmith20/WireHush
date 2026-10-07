/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package manager

import (
	"golang.zx2c4.com/wireguard/windows/conf"
	"golang.zx2c4.com/wireguard/windows/driver"
)

func wireHushStoredConfigFromRecord(record conf.TunnelRecord) (*conf.Config, error) {
	if err := record.Validate(); err != nil {
		return nil, err
	}
	return conf.FromWgQuick(record.WGQuickText, record.Name)
}

func wireHushStoredConfig(locator conf.TunnelServiceLocator) (*conf.Config, error) {
	if err := locator.Validate(); err != nil {
		return nil, err
	}
	record, err := conf.LoadTunnelRecord(locator.Scope, locator.OwnerSID, locator.TunnelID)
	if err != nil {
		return nil, err
	}
	return wireHushStoredConfigFromRecord(record)
}

func wireHushRuntimeAdapterName(locator conf.TunnelServiceLocator) (string, error) {
	if err := locator.Validate(); err != nil {
		return "", err
	}
	return conf.WireHushAdapterNameOfTunnelID(locator.TunnelID)
}

func wireHushRuntimeConfig(locator conf.TunnelServiceLocator) (*conf.Config, error) {
	if err := locator.Validate(); err != nil {
		return nil, err
	}
	storedConfig, err := wireHushStoredConfig(locator)
	if err != nil {
		return nil, err
	}
	adapterName, err := wireHushRuntimeAdapterName(locator)
	if err != nil {
		return nil, err
	}
	adapter, err := driver.OpenAdapter(adapterName)
	if err != nil {
		return nil, err
	}
	defer adapter.Close()
	runtimeConfig, err := adapter.Configuration()
	if err != nil {
		return nil, err
	}
	return conf.FromDriverConfiguration(runtimeConfig, storedConfig), nil
}
