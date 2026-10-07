/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package manager

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"

	"golang.zx2c4.com/wireguard/windows/conf"
)

type wireHushCaller struct {
	SID           string
	Administrator bool
	WireHushUser  bool
}

func (caller wireHushCaller) Validate() error {
	if caller.SID == "" {
		return errors.New("WireHush caller SID is required")
	}
	sid, err := windows.StringToSid(caller.SID)
	if err != nil {
		return fmt.Errorf("WireHush caller SID is not valid: %w", err)
	}
	if sid.String() != caller.SID {
		return errors.New("WireHush caller SID is not canonical")
	}
	return nil
}

type wireHushTunnelOperation uint8

const (
	wireHushTunnelMetadataRead wireHushTunnelOperation = iota + 1
	wireHushTunnelStoredConfigRead
	wireHushTunnelRuntimeConfigRead
	wireHushTunnelControl
	wireHushTunnelRecordMutation
)

var errWireHushAccessDenied = errors.New("WireHush access denied")

func (operation wireHushTunnelOperation) Validate() error {
	switch operation {
	case wireHushTunnelMetadataRead, wireHushTunnelStoredConfigRead, wireHushTunnelRuntimeConfigRead, wireHushTunnelControl, wireHushTunnelRecordMutation:
		return nil
	default:
		return fmt.Errorf("invalid WireHush tunnel operation %d", operation)
	}
}

func authorizeWireHushTunnel(caller wireHushCaller, locator conf.TunnelServiceLocator, operation wireHushTunnelOperation) error {
	if err := caller.Validate(); err != nil {
		return err
	}
	if err := locator.Validate(); err != nil {
		return err
	}
	if err := operation.Validate(); err != nil {
		return err
	}
	return authorizeWireHushNamespacePolicy(caller, locator.Scope, locator.OwnerSID, operation)
}

func authorizeWireHushNamespace(caller wireHushCaller, scope conf.TunnelScope, ownerSID string, operation wireHushTunnelOperation) error {
	if err := caller.Validate(); err != nil {
		return err
	}
	if err := validateWireHushNamespace(scope, ownerSID); err != nil {
		return err
	}
	if err := operation.Validate(); err != nil {
		return err
	}
	return authorizeWireHushNamespacePolicy(caller, scope, ownerSID, operation)
}

func validateWireHushNamespace(scope conf.TunnelScope, ownerSID string) error {
	switch scope {
	case conf.TunnelScopePrivate:
		if ownerSID == "" {
			return errors.New("private WireHush namespace owner SID is required")
		}
		sid, err := windows.StringToSid(ownerSID)
		if err != nil {
			return fmt.Errorf("private WireHush namespace owner SID is not valid: %w", err)
		}
		if sid.String() != ownerSID {
			return errors.New("private WireHush namespace owner SID is not canonical")
		}
	case conf.TunnelScopeShared:
		if ownerSID != "" {
			return errors.New("shared WireHush namespace owner SID must be empty")
		}
	default:
		return fmt.Errorf("invalid WireHush namespace scope %d", scope)
	}
	return nil
}

func authorizeWireHushNamespacePolicy(caller wireHushCaller, scope conf.TunnelScope, ownerSID string, operation wireHushTunnelOperation) error {
	if scope == conf.TunnelScopePrivate {
		if caller.SID == ownerSID {
			return nil
		}
		return fmt.Errorf("%w: private namespace belongs to %q", errWireHushAccessDenied, ownerSID)
	}
	if caller.Administrator {
		return nil
	}
	if caller.WireHushUser && (operation == wireHushTunnelMetadataRead || operation == wireHushTunnelControl) {
		return nil
	}
	return fmt.Errorf("%w: shared namespace operation %d", errWireHushAccessDenied, operation)
}
