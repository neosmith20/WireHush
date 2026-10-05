/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package tunnel

import (
	"errors"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.zx2c4.com/wireguard/windows/conf"
)

func RunWireHush(locator conf.TunnelServiceLocator) error {
	if err := locator.Validate(); err != nil {
		return err
	}
	serviceName, err := conf.ServiceNameOfTunnelID(locator.TunnelID)
	if err != nil {
		return err
	}
	return svc.Run(serviceName, &tunnelService{RecordLocator: &locator})
}

func wireHushRuntimeFromRecord(record conf.TunnelRecord) (*conf.Config, wireHushAdapterIdentity, error) {
	if err := record.Validate(); err != nil {
		return nil, wireHushAdapterIdentity{}, err
	}
	config, err := conf.FromWgQuick(record.WGQuickText, record.Name)
	if err != nil {
		return nil, wireHushAdapterIdentity{}, err
	}
	identity, err := wireHushAdapterIdentityForTunnelID(record.TunnelID)
	if err != nil {
		return nil, wireHushAdapterIdentity{}, err
	}
	return config, identity, nil
}

func (service *tunnelService) prepareSource() (*conf.Config, string, *windows.GUID, error) {
	if (service.Path == "") == (service.RecordLocator == nil) {
		return nil, "", nil, errors.New("tunnel service must have exactly one source")
	}
	if service.RecordLocator == nil {
		config, err := conf.LoadFromPath(service.Path)
		if err != nil {
			return nil, "", nil, err
		}
		return config, config.Name, deterministicGUID(config), nil
	}
	record, err := conf.LoadTunnelRecord(service.RecordLocator.Scope, service.RecordLocator.OwnerSID, service.RecordLocator.TunnelID)
	if err != nil {
		return nil, "", nil, err
	}
	config, identity, err := wireHushRuntimeFromRecord(record)
	if err != nil {
		return nil, "", nil, err
	}
	return config, identity.Name, &identity.GUID, nil
}
