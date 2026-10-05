/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package conf

import (
	"errors"
	"strings"
	"sync"

	"golang.org/x/sys/windows"
)

var (
	// ErrTunnelNameInUse reports a requested case-insensitive name conflict.
	ErrTunnelNameInUse = errors.New("tunnel name is already in use")
	// ErrTunnelNamespaceNameConflict reports an existing invalid namespace.
	ErrTunnelNamespaceNameConflict = errors.New("tunnel namespace has duplicate names")

	// tunnelRecordMutationMu serializes supported TunnelRecord mutations in one
	// process. WireHush Manager is the sole supported production writer; adding
	// another writer requires architecture review before it is introduced.
	tunnelRecordMutationMu sync.Mutex
)

func saveTunnelRecordMutationAtRoot(root string, record TunnelRecord, overwrite bool, directorySD *windows.SECURITY_DESCRIPTOR) error {
	if err := record.Validate(); err != nil {
		return err
	}
	tunnelRecordMutationMu.Lock()
	defer tunnelRecordMutationMu.Unlock()
	records, err := listTunnelRecordsAtRoot(root, record.Scope, record.OwnerSID)
	if err != nil {
		return err
	}
	if err := validateTunnelRecordNameMutation(records, record); err != nil {
		return err
	}
	return saveTunnelRecordAtRoot(root, record, overwrite, directorySD)
}

func deleteTunnelRecordMutationAtRoot(root string, scope TunnelScope, ownerSID string, id TunnelID) error {
	tunnelRecordMutationMu.Lock()
	defer tunnelRecordMutationMu.Unlock()
	return deleteTunnelRecordAtRoot(root, scope, ownerSID, id)
}

func validateTunnelRecordNameMutation(records []TunnelRecord, candidate TunnelRecord) error {
	remaining := make([]TunnelRecord, 0, len(records))
	for _, record := range records {
		if record.TunnelID != candidate.TunnelID {
			remaining = append(remaining, record)
		}
	}
	for index, record := range remaining {
		for _, other := range remaining[:index] {
			if strings.EqualFold(record.Name, other.Name) {
				return ErrTunnelNamespaceNameConflict
			}
		}
	}
	for _, record := range remaining {
		if strings.EqualFold(record.Name, candidate.Name) {
			return ErrTunnelNameInUse
		}
	}
	return nil
}
