/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */
package manager

import (
	"context"
	"encoding/json"
	"errors"
	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/windows/conf"
	"os"
	"strings"
)

type wireHushMigrationEntry struct {
	Product  string        `json:"product"`
	FileName string        `json:"file_name"`
	Digest   string        `json:"source_digest"`
	TunnelID conf.TunnelID `json:"tunnel_id"`
	Complete bool          `json:"complete"`
}
type wireHushMigrationJournal struct {
	Version uint32                   `json:"version"`
	Entries []wireHushMigrationEntry `json:"entries"`
}
type wireHushMigrationStore struct {
	readJournal  func() ([]byte, error)
	writeJournal func([]byte) error
	backup       func(conf.LegacyMigrationSource) error
	create       func(conf.TunnelRecord, bool) error
	load         func(conf.TunnelScope, string, conf.TunnelID) (conf.TunnelRecord, error)
	newID        func() (conf.TunnelID, error)
}

func wireHushMigrationKey(product, file string) string { return product + "/" + strings.ToLower(file) }
func runWireHushMigration(ctx context.Context, sources []conf.LegacyMigrationSource, store wireHushMigrationStore) error {
	journal := wireHushMigrationJournal{Version: 1}
	if data, err := store.readJournal(); err == nil {
		if err := json.Unmarshal(data, &journal); err != nil || journal.Version != 1 || len(journal.Entries) > 4096 {
			return errors.New("migration journal requires repair")
		}
	} else if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
		return err
	}
	entries := make(map[string]int)
	ids := make(map[conf.TunnelID]bool)
	for index, entry := range journal.Entries {
		if entry.Product != "TunnelMint" && entry.Product != "WireHush" {
			return errors.New("invalid migration source identity")
		}
		if _, err := conf.NameFromPath(entry.FileName); err != nil || strings.ContainsAny(entry.FileName, `\/:`) {
			return errors.New("invalid migration source filename")
		}
		if !entry.TunnelID.Valid() || ids[entry.TunnelID] {
			return errors.New("invalid migration target identity")
		}
		ids[entry.TunnelID] = true
		key := wireHushMigrationKey(entry.Product, entry.FileName)
		if _, found := entries[key]; found {
			return errors.New("duplicate migration source identity")
		}
		entries[key] = index
	}
	saveJournal := func() error {
		data, err := json.Marshal(journal)
		if err != nil {
			return err
		}
		return store.writeJournal(data)
	}
	seen := make(map[string]bool)
	for _, source := range sources {
		if err := ctx.Err(); err != nil {
			return err
		}
		if source.Product != "TunnelMint" && source.Product != "WireHush" {
			return errors.New("invalid legacy source")
		}
		name, err := conf.NameFromPath(source.FileName)
		if err != nil || name != source.Name || strings.ContainsAny(source.FileName, `\/:`) {
			return errors.New("invalid legacy source filename")
		}
		if _, err := conf.FromWgQuick(source.WGQuickText, source.Name); err != nil {
			return err
		}
		key := wireHushMigrationKey(source.Product, source.FileName)
		seen[key] = true
		index, found := entries[key]
		if found && journal.Entries[index].Complete {
			continue
		} // Preserve subsequent owner edits/deletions.
		if err := store.backup(source); err != nil {
			return err
		}
		if !found {
			id, err := store.newID()
			if err != nil {
				return err
			}
			index = len(journal.Entries)
			journal.Entries = append(journal.Entries, wireHushMigrationEntry{Product: source.Product, FileName: source.FileName, Digest: source.Digest, TunnelID: id})
			entries[key] = index
			// Persist the reserved ID before creating a target. A crash between target
			// creation and completion cannot generate a second identity on retry.
			if err := saveJournal(); err != nil {
				return err
			}
		}
		entry := journal.Entries[index]
		if entry.Digest != source.Digest {
			return errors.New("legacy source changed during incomplete migration")
		}
		desired := conf.TunnelRecord{TunnelID: entry.TunnelID, Scope: conf.TunnelScopeShared, Name: source.Name, WGQuickText: source.WGQuickText}
		existing, err := store.load(conf.TunnelScopeShared, "", entry.TunnelID)
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
			if err := store.create(desired, false); err != nil {
				return err
			}
			existing, err = store.load(conf.TunnelScopeShared, "", entry.TunnelID)
		}
		if err != nil {
			return err
		}
		if existing != desired {
			return errors.New("migrated record failed identity and content verification")
		}
		journal.Entries[index].Complete = true
		if err := saveJournal(); err != nil {
			return err
		}
	}
	for _, entry := range journal.Entries {
		if !entry.Complete && !seen[wireHushMigrationKey(entry.Product, entry.FileName)] {
			return errors.New("unfinished migration requires its preserved legacy source")
		}
	}
	return nil
}

// Invoked only by the elevated installer's deferred LocalSystem action, never
// through the UI RPC surface. All source paths are derived from known folders.
func MigrateLegacyV1(ctx context.Context) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	if user.User.Sid.String() != "S-1-5-18" {
		return errWireHushAccessDenied
	}
	if _, err := conf.PrepareWireHushMachineData(); err != nil {
		return err
	}
	sources, err := conf.ReadWireHushLegacyMigrationSources()
	if err != nil {
		return err
	}
	// Keep encrypted copies before stopping legacy processes or allowing the
	// predecessor MSI to remove its legacy Program Files data during upgrade.
	for _, source := range sources {
		if err := conf.BackupWireHushLegacySource(source); err != nil {
			return err
		}
	}
	if err := stopVerifiedWireHushLegacyServices(ctx); err != nil {
		return err
	}
	sources, err = conf.ReadWireHushLegacyMigrationSources()
	if err != nil {
		return err
	}
	store := wireHushMigrationStore{readJournal: conf.LoadWireHushMigrationJournal, writeJournal: conf.SaveWireHushMigrationJournal, backup: conf.BackupWireHushLegacySource, create: conf.SaveTunnelRecord, load: conf.LoadTunnelRecord, newID: conf.NewTunnelID}
	if err := runWireHushMigration(ctx, sources, store); err != nil {
		return err
	}
	if err := conf.MigrateWireHushLegacyBootstrapSettings(); err != nil {
		return err
	}
	// Service registrations remain available for MSI rollback until commit.
	return nil
}
