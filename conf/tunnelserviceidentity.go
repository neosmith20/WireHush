/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package conf

import (
	"errors"

	"golang.zx2c4.com/wireguard/windows/product"
)

const WireHushTunnelServiceCommand = "/wirehushtunnelservice"

type TunnelServiceLocator struct {
	Scope    TunnelScope
	OwnerSID string
	TunnelID TunnelID
}

func (locator TunnelServiceLocator) Validate() error {
	if !locator.TunnelID.Valid() {
		return errors.New("TunnelID is not valid")
	}
	_, err := tunnelRecordDirectoryFromRoot("", locator.Scope, locator.OwnerSID)
	return err
}

func WireHushTunnelServiceArgs(locator TunnelServiceLocator) ([]string, error) {
	if err := locator.Validate(); err != nil {
		return nil, err
	}
	if locator.Scope == TunnelScopePrivate {
		return []string{WireHushTunnelServiceCommand, "private", locator.OwnerSID, locator.TunnelID.String()}, nil
	}
	return []string{WireHushTunnelServiceCommand, "shared", locator.TunnelID.String()}, nil
}

func ParseWireHushTunnelServiceArgs(args []string) (TunnelServiceLocator, error) {
	if len(args) < 2 || args[0] != WireHushTunnelServiceCommand {
		return TunnelServiceLocator{}, errors.New("invalid WireHush tunnel service command")
	}
	var locator TunnelServiceLocator
	switch args[1] {
	case "private":
		if len(args) != 4 {
			return TunnelServiceLocator{}, errors.New("invalid private WireHush tunnel service arguments")
		}
		id, err := ParseTunnelID(args[3])
		if err != nil {
			return TunnelServiceLocator{}, err
		}
		locator = TunnelServiceLocator{Scope: TunnelScopePrivate, OwnerSID: args[2], TunnelID: id}
	case "shared":
		if len(args) != 3 {
			return TunnelServiceLocator{}, errors.New("invalid shared WireHush tunnel service arguments")
		}
		id, err := ParseTunnelID(args[2])
		if err != nil {
			return TunnelServiceLocator{}, err
		}
		locator = TunnelServiceLocator{Scope: TunnelScopeShared, TunnelID: id}
	default:
		return TunnelServiceLocator{}, errors.New("invalid WireHush tunnel service scope")
	}
	return locator, locator.Validate()
}

func ServiceNameOfTunnelID(id TunnelID) (string, error) {
	if !id.Valid() {
		return "", errors.New("TunnelID is not valid")
	}
	return product.TunnelServicePrefix + id.Token(), nil
}
