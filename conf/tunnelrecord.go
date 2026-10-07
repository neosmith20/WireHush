/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package conf

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"golang.org/x/sys/windows"
)

const TunnelRecordStorageVersion = 1

// TunnelScope identifies whether a tunnel belongs to one user or is shared.
// Its zero value is invalid.
type TunnelScope uint8

const (
	TunnelScopePrivate TunnelScope = iota + 1
	TunnelScopeShared
)

func (scope TunnelScope) String() string {
	switch scope {
	case TunnelScopePrivate:
		return "private"
	case TunnelScopeShared:
		return "shared"
	default:
		return ""
	}
}

// TunnelRecord is the validated current-version WireHush tunnel state.
type TunnelRecord struct {
	TunnelID    TunnelID
	Name        string
	Scope       TunnelScope
	OwnerSID    string
	WGQuickText string
}

// Validate verifies the structural validity of a tunnel record.
func (record TunnelRecord) Validate() error {
	if !record.TunnelID.Valid() {
		return errors.New("TunnelID is not valid")
	}
	if !TunnelNameIsValid(record.Name) {
		return errors.New("tunnel name is not valid")
	}
	switch record.Scope {
	case TunnelScopePrivate:
		if record.OwnerSID == "" {
			return errors.New("private tunnel owner SID is required")
		}
		sid, err := windows.StringToSid(record.OwnerSID)
		if err != nil {
			return fmt.Errorf("private tunnel owner SID is not valid: %w", err)
		}
		if sid.String() != record.OwnerSID {
			return errors.New("private tunnel owner SID is not canonical")
		}
	case TunnelScopeShared:
		if record.OwnerSID != "" {
			return errors.New("shared tunnel owner SID must be empty")
		}
	default:
		return errors.New("tunnel scope is not valid")
	}
	if record.WGQuickText == "" {
		return errors.New("wg-quick text is required")
	}
	if _, err := FromWgQuick(record.WGQuickText, record.Name); err != nil {
		return fmt.Errorf("wg-quick text is not valid: %w", err)
	}
	return nil
}

type tunnelRecordWire struct {
	StorageVersion *int    `json:"storage_version"`
	TunnelID       *string `json:"tunnel_id"`
	Name           *string `json:"name"`
	Scope          *string `json:"scope"`
	OwnerSID       *string `json:"owner_sid"`
	WGQuickText    *string `json:"wg_quick_text"`
}

// MarshalTunnelRecord serializes a validated record in the current storage format.
func MarshalTunnelRecord(record TunnelRecord) ([]byte, error) {
	if err := record.Validate(); err != nil {
		return nil, err
	}
	storageVersion := TunnelRecordStorageVersion
	tunnelID := record.TunnelID.String()
	name := record.Name
	scope := record.Scope.String()
	ownerSID := record.OwnerSID
	wgQuickText := record.WGQuickText
	return json.Marshal(tunnelRecordWire{
		StorageVersion: &storageVersion,
		TunnelID:       &tunnelID,
		Name:           &name,
		Scope:          &scope,
		OwnerSID:       &ownerSID,
		WGQuickText:    &wgQuickText,
	})
}

// ParseTunnelRecord parses one complete current-version tunnel record.
func ParseTunnelRecord(data []byte) (TunnelRecord, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var wire tunnelRecordWire
	if err := decoder.Decode(&wire); err != nil {
		return TunnelRecord{}, fmt.Errorf("invalid tunnel record JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return TunnelRecord{}, errors.New("tunnel record JSON has trailing data")
		}
		return TunnelRecord{}, fmt.Errorf("invalid trailing tunnel record JSON: %w", err)
	}
	if wire.StorageVersion == nil || wire.TunnelID == nil || wire.Name == nil || wire.Scope == nil || wire.OwnerSID == nil || wire.WGQuickText == nil {
		return TunnelRecord{}, errors.New("tunnel record JSON is missing a required field")
	}
	if *wire.StorageVersion != TunnelRecordStorageVersion {
		return TunnelRecord{}, fmt.Errorf("unsupported tunnel record storage version %d", *wire.StorageVersion)
	}
	id, err := ParseTunnelID(*wire.TunnelID)
	if err != nil {
		return TunnelRecord{}, fmt.Errorf("invalid tunnel record TunnelID: %w", err)
	}
	scope, err := parseTunnelScope(*wire.Scope)
	if err != nil {
		return TunnelRecord{}, err
	}
	record := TunnelRecord{
		TunnelID:    id,
		Name:        *wire.Name,
		Scope:       scope,
		OwnerSID:    *wire.OwnerSID,
		WGQuickText: *wire.WGQuickText,
	}
	if err := record.Validate(); err != nil {
		return TunnelRecord{}, err
	}
	return record, nil
}

func parseTunnelScope(text string) (TunnelScope, error) {
	switch text {
	case "private":
		return TunnelScopePrivate, nil
	case "shared":
		return TunnelScopeShared, nil
	default:
		return 0, errors.New("tunnel scope is not valid")
	}
}
