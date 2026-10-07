/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package manager

import (
	"context"
	"errors"

	"golang.org/x/sys/windows"

	"golang.zx2c4.com/wireguard/windows/conf"
)

type wireHushTunnelMetadata struct {
	TunnelID conf.TunnelID
	Name     string
	Scope    conf.TunnelScope
	OwnerSID string
}

type wireHushManagerControl struct {
	loadRecord    func(conf.TunnelScope, string, conf.TunnelID) (conf.TunnelRecord, error)
	listRecords   func(conf.TunnelScope, string) ([]conf.TunnelRecord, error)
	saveRecord    func(conf.TunnelRecord, bool) error
	deleteRecord  func(conf.TunnelScope, string, conf.TunnelID) error
	storedConfig  func(conf.TunnelServiceLocator) (*conf.Config, error)
	runtimeConfig func(conf.TunnelServiceLocator) (*conf.Config, error)
	start         func(conf.TunnelServiceLocator) error
	stop          func(conf.TunnelServiceLocator) error
	waitForStop   func(conf.TunnelServiceLocator) error
	state         func(conf.TunnelServiceLocator) (TunnelState, error)
	startContext  func(context.Context, conf.TunnelServiceLocator) error
	waitContext   func(context.Context, conf.TunnelServiceLocator) error
}

func newWireHushManagerControl() wireHushManagerControl {
	return wireHushManagerControl{
		loadRecord:    conf.LoadTunnelRecord,
		listRecords:   conf.ListTunnelRecords,
		saveRecord:    conf.SaveTunnelRecord,
		deleteRecord:  conf.DeleteTunnelRecord,
		storedConfig:  wireHushStoredConfig,
		runtimeConfig: wireHushRuntimeConfig,
		start:         InstallWireHushTunnel,
		stop:          UninstallWireHushTunnel,
		waitForStop:   WaitForWireHushTunnelStop,
		state:         WireHushTunnelState,
		startContext:  InstallWireHushTunnelContext,
		waitContext:   WaitForWireHushTunnelStopContext,
	}
}

func (control wireHushManagerControl) withContext(ctx context.Context) wireHushManagerControl {
	if control.startContext != nil {
		control.start = func(locator conf.TunnelServiceLocator) error { return control.startContext(ctx, locator) }
	}
	if control.waitContext != nil {
		control.waitForStop = func(locator conf.TunnelServiceLocator) error { return control.waitContext(ctx, locator) }
	}
	return control
}

func authorizeWireHushRecordScripts(caller wireHushCaller, record conf.TunnelRecord) error {
	config, err := wireHushStoredConfigFromRecord(record)
	if err != nil {
		return err
	}
	if !caller.Administrator && (config.Interface.PreUp != "" || config.Interface.PostUp != "" || config.Interface.PreDown != "" || config.Interface.PostDown != "") {
		return errWireHushAccessDenied
	}
	return nil
}

func wireHushMetadataFromRecord(record conf.TunnelRecord) wireHushTunnelMetadata {
	return wireHushTunnelMetadata{
		TunnelID: record.TunnelID,
		Name:     record.Name,
		Scope:    record.Scope,
		OwnerSID: record.OwnerSID,
	}
}

func wireHushLocatorFromRecord(record conf.TunnelRecord) (conf.TunnelServiceLocator, error) {
	if err := record.Validate(); err != nil {
		return conf.TunnelServiceLocator{}, err
	}
	return conf.TunnelServiceLocator{Scope: record.Scope, OwnerSID: record.OwnerSID, TunnelID: record.TunnelID}, nil
}

func (control wireHushManagerControl) resolveAuthorizedTunnel(caller wireHushCaller, requested conf.TunnelServiceLocator, operation wireHushTunnelOperation) (conf.TunnelRecord, conf.TunnelServiceLocator, error) {
	if err := authorizeWireHushTunnel(caller, requested, operation); err != nil {
		return conf.TunnelRecord{}, conf.TunnelServiceLocator{}, err
	}
	record, err := control.loadRecord(requested.Scope, requested.OwnerSID, requested.TunnelID)
	if err != nil {
		return conf.TunnelRecord{}, conf.TunnelServiceLocator{}, err
	}
	canonical, err := wireHushLocatorFromRecord(record)
	if err != nil {
		return conf.TunnelRecord{}, conf.TunnelServiceLocator{}, err
	}
	if canonical != requested {
		return conf.TunnelRecord{}, conf.TunnelServiceLocator{}, errors.New("stored tunnel record identity does not match requested locator")
	}
	if err := authorizeWireHushTunnel(caller, canonical, operation); err != nil {
		return conf.TunnelRecord{}, conf.TunnelServiceLocator{}, err
	}
	return record, canonical, nil
}

func (control wireHushManagerControl) ListTunnelMetadata(caller wireHushCaller, scope conf.TunnelScope, ownerSID string) ([]wireHushTunnelMetadata, error) {
	if err := authorizeWireHushNamespace(caller, scope, ownerSID, wireHushTunnelMetadataRead); err != nil {
		return nil, err
	}
	records, err := control.listRecords(scope, ownerSID)
	if err != nil {
		return nil, err
	}
	metadata := make([]wireHushTunnelMetadata, len(records))
	for index, record := range records {
		locator, err := wireHushLocatorFromRecord(record)
		if err != nil {
			return nil, err
		}
		if locator.Scope != scope || locator.OwnerSID != ownerSID {
			return nil, errors.New("stored tunnel record identity does not match requested namespace")
		}
		if err := authorizeWireHushTunnel(caller, locator, wireHushTunnelMetadataRead); err != nil {
			return nil, err
		}
		metadata[index] = wireHushMetadataFromRecord(record)
	}
	return metadata, nil
}

func (control wireHushManagerControl) TunnelMetadata(caller wireHushCaller, locator conf.TunnelServiceLocator) (wireHushTunnelMetadata, error) {
	record, _, err := control.resolveAuthorizedTunnel(caller, locator, wireHushTunnelMetadataRead)
	if err != nil {
		return wireHushTunnelMetadata{}, err
	}
	return wireHushMetadataFromRecord(record), nil
}

func (control wireHushManagerControl) TunnelState(caller wireHushCaller, locator conf.TunnelServiceLocator) (TunnelState, error) {
	_, canonical, err := control.resolveAuthorizedTunnel(caller, locator, wireHushTunnelMetadataRead)
	if err != nil {
		return TunnelUnknown, err
	}
	return control.state(canonical)
}

func (control wireHushManagerControl) StoredTunnelConfig(caller wireHushCaller, locator conf.TunnelServiceLocator) (*conf.Config, error) {
	_, canonical, err := control.resolveAuthorizedTunnel(caller, locator, wireHushTunnelStoredConfigRead)
	if err != nil {
		return nil, err
	}
	return control.storedConfig(canonical)
}

func (control wireHushManagerControl) RuntimeTunnelConfig(caller wireHushCaller, locator conf.TunnelServiceLocator) (*conf.Config, error) {
	_, canonical, err := control.resolveAuthorizedTunnel(caller, locator, wireHushTunnelRuntimeConfigRead)
	if err != nil {
		return nil, err
	}
	return control.runtimeConfig(canonical)
}

func (control wireHushManagerControl) StartTunnel(caller wireHushCaller, locator conf.TunnelServiceLocator) error {
	record, canonical, err := control.resolveAuthorizedTunnel(caller, locator, wireHushTunnelControl)
	if err != nil {
		return err
	}
	if err := authorizeWireHushRecordScripts(caller, record); err != nil {
		return err
	}
	return control.start(canonical)
}

func (control wireHushManagerControl) StopTunnel(caller wireHushCaller, locator conf.TunnelServiceLocator) error {
	_, canonical, err := control.resolveAuthorizedTunnel(caller, locator, wireHushTunnelControl)
	if err != nil {
		return err
	}
	return control.stopAndWait(canonical)
}

func (control wireHushManagerControl) stopAndWait(locator conf.TunnelServiceLocator) error {
	if err := control.stop(locator); err != nil && !errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return err
	}
	return control.waitForStop(locator)
}

func (control wireHushManagerControl) WaitForTunnelStop(caller wireHushCaller, locator conf.TunnelServiceLocator) error {
	_, canonical, err := control.resolveAuthorizedTunnel(caller, locator, wireHushTunnelControl)
	if err != nil {
		return err
	}
	return control.waitForStop(canonical)
}

func (control wireHushManagerControl) CreateTunnelRecord(caller wireHushCaller, record conf.TunnelRecord) error {
	candidate, err := wireHushLocatorFromRecord(record)
	if err != nil {
		return err
	}
	if err := authorizeWireHushTunnel(caller, candidate, wireHushTunnelRecordMutation); err != nil {
		return err
	}
	if err := authorizeWireHushRecordScripts(caller, record); err != nil {
		return err
	}
	return control.saveRecord(record, false)
}

func (control wireHushManagerControl) SaveTunnelRecord(caller wireHushCaller, locator conf.TunnelServiceLocator, record conf.TunnelRecord) error {
	_, canonical, err := control.resolveAuthorizedTunnel(caller, locator, wireHushTunnelRecordMutation)
	if err != nil {
		return err
	}
	candidate, err := wireHushLocatorFromRecord(record)
	if err != nil {
		return err
	}
	if candidate != canonical {
		return errors.New("tunnel record identity cannot change during update")
	}
	if err := authorizeWireHushRecordScripts(caller, record); err != nil {
		return err
	}
	return control.saveRecord(record, true)
}

func (control wireHushManagerControl) DeleteTunnelRecord(caller wireHushCaller, locator conf.TunnelServiceLocator) error {
	_, canonical, err := control.resolveAuthorizedTunnel(caller, locator, wireHushTunnelRecordMutation)
	if err != nil {
		return err
	}
	if err := control.stopAndWait(canonical); err != nil {
		return err
	}
	return control.deleteRecord(canonical.Scope, canonical.OwnerSID, canonical.TunnelID)
}
