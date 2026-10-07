/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package manager

import (
	"context"
	"errors"

	"golang.zx2c4.com/wireguard/windows/conf"
)

var (
	errWireHushDeviceBusy   = errors.New("WireHush is currently active; stop the active tunnel first")
	errWireHushTunnelActive = errors.New("stop the tunnel before editing its configuration")
	errWireHushClosing      = errors.New("WireHush is closing")
)

// One instance is shared by all production RPC connections. The gate protects
// both SCM admission and storage mutations; a new gate per connection is unsafe.
type wireHushMutations struct {
	control   wireHushManagerControl
	gate      chan struct{}
	inventory func() ([]wireHushTrackedTunnel, error)
	closing   bool // protected by gate
}

func newWireHushMutations() *wireHushMutations {
	return &wireHushMutations{
		control:   newWireHushManagerControl(),
		gate:      make(chan struct{}, 1),
		inventory: inventoryWireHushServices,
	}
}

func (mutations *wireHushMutations) enter(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case mutations.gate <- struct{}{}:
		if err := ctx.Err(); err != nil {
			mutations.leave()
			return err
		}
		if mutations.closing {
			mutations.leave()
			return errWireHushClosing
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (mutations *wireHushMutations) leave() { <-mutations.gate }

func (mutations *wireHushMutations) Start(ctx context.Context, caller wireHushCaller, locator conf.TunnelServiceLocator) error {
	if err := authorizeWireHushTunnel(caller, locator, wireHushTunnelControl); err != nil {
		return err
	}
	if err := mutations.enter(ctx); err != nil {
		return err
	}
	defer mutations.leave()
	_, canonical, err := mutations.control.resolveAuthorizedTunnel(caller, locator, wireHushTunnelControl)
	if err != nil {
		return err
	}
	active, err := mutations.inventory()
	if err != nil {
		return err
	}
	alreadyActive := false
	for _, tunnel := range active {
		if tunnel.State == TunnelStopped {
			continue
		}
		if tunnel.Locator != canonical {
			return errWireHushDeviceBusy
		}
		if tunnel.State == TunnelStarted || tunnel.State == TunnelStarting {
			alreadyActive = true
			continue
		}
		return errWireHushDeviceBusy
	}
	if alreadyActive {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return mutations.control.start(canonical)
}

func (mutations *wireHushMutations) Stop(ctx context.Context, caller wireHushCaller, locator conf.TunnelServiceLocator) error {
	if err := authorizeWireHushTunnel(caller, locator, wireHushTunnelControl); err != nil {
		return err
	}
	if err := mutations.enter(ctx); err != nil {
		return err
	}
	defer mutations.leave()
	return mutations.control.StopTunnel(caller, locator)
}

func (mutations *wireHushMutations) Create(ctx context.Context, caller wireHushCaller, scope conf.TunnelScope, name, text string) (wireHushTunnelMetadata, error) {
	owner := ""
	if scope == conf.TunnelScopePrivate {
		owner = caller.SID
	}
	if err := authorizeWireHushNamespace(caller, scope, owner, wireHushTunnelRecordMutation); err != nil {
		return wireHushTunnelMetadata{}, err
	}
	if err := mutations.enter(ctx); err != nil {
		return wireHushTunnelMetadata{}, err
	}
	defer mutations.leave()
	id, err := conf.NewTunnelID()
	if err != nil {
		return wireHushTunnelMetadata{}, err
	}
	record := conf.TunnelRecord{TunnelID: id, Scope: scope, OwnerSID: owner, Name: name, WGQuickText: text}
	if err := mutations.control.CreateTunnelRecord(caller, record); err != nil {
		return wireHushTunnelMetadata{}, err
	}
	return wireHushMetadataFromRecord(record), nil
}

func (mutations *wireHushMutations) Update(ctx context.Context, caller wireHushCaller, locator conf.TunnelServiceLocator, name, text string) error {
	if err := authorizeWireHushTunnel(caller, locator, wireHushTunnelRecordMutation); err != nil {
		return err
	}
	if err := mutations.enter(ctx); err != nil {
		return err
	}
	defer mutations.leave()
	record, canonical, err := mutations.control.resolveAuthorizedTunnel(caller, locator, wireHushTunnelRecordMutation)
	if err != nil {
		return err
	}
	state, err := mutations.control.state(canonical)
	if err != nil {
		return err
	}
	if state != TunnelStopped {
		return errWireHushTunnelActive
	}
	record.Name, record.WGQuickText = name, text
	return mutations.control.SaveTunnelRecord(caller, canonical, record)
}

func (mutations *wireHushMutations) Delete(ctx context.Context, caller wireHushCaller, locator conf.TunnelServiceLocator) error {
	if err := authorizeWireHushTunnel(caller, locator, wireHushTunnelRecordMutation); err != nil {
		return err
	}
	if err := mutations.enter(ctx); err != nil {
		return err
	}
	defer mutations.leave()
	return mutations.control.DeleteTunnelRecord(caller, locator)
}
