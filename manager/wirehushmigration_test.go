/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */
package manager

import (
	"context"
	"encoding/json"
	"errors"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
	"golang.zx2c4.com/wireguard/windows/conf"
	"os"
	"testing"
)

func testWireHushMigrationStore(t *testing.T) (wireHushMigrationStore, *[]byte, map[conf.TunnelID]conf.TunnelRecord, *int) {
	t.Helper()
	var journal []byte
	records := make(map[conf.TunnelID]conf.TunnelRecord)
	created := new(int)
	store := wireHushMigrationStore{
		readJournal: func() ([]byte, error) {
			if journal == nil {
				return nil, os.ErrNotExist
			}
			return journal, nil
		}, writeJournal: func(data []byte) error { journal = append([]byte(nil), data...); return nil },
		backup: func(conf.LegacyMigrationSource) error { return nil }, newID: conf.NewTunnelID,
		create: func(record conf.TunnelRecord, overwrite bool) error {
			if overwrite {
				t.Fatal("migration overwrote an existing record")
			}
			if record.Scope != conf.TunnelScopeShared || record.OwnerSID != "" {
				t.Fatal("legacy migration changed namespace authority")
			}
			records[record.TunnelID] = record
			*created++
			return nil
		},
		load: func(_ conf.TunnelScope, _ string, id conf.TunnelID) (conf.TunnelRecord, error) {
			record, found := records[id]
			if !found {
				return conf.TunnelRecord{}, os.ErrNotExist
			}
			return record, nil
		},
	}
	return store, &journal, records, created
}
func migrationSource() conf.LegacyMigrationSource {
	return conf.LegacyMigrationSource{Product: "TunnelMint", FileName: "Home.conf.dpapi", Name: "Home", Digest: "digest", WGQuickText: wireHushControlWGQuick}
}
func TestWireHushMigrationIsIdempotentAndPreservesOwnerChanges(t *testing.T) {
	store, journal, records, created := testWireHushMigrationStore(t)
	source := migrationSource()
	if err := runWireHushMigration(context.Background(), []conf.LegacyMigrationSource{source}, store); err != nil {
		t.Fatal(err)
	}
	for id, record := range records {
		record.Name = "Owner renamed"
		records[id] = record
	}
	if err := runWireHushMigration(context.Background(), []conf.LegacyMigrationSource{source}, store); err != nil {
		t.Fatal(err)
	}
	for id := range records {
		delete(records, id)
	}
	if err := runWireHushMigration(context.Background(), []conf.LegacyMigrationSource{source}, store); err != nil {
		t.Fatal(err)
	}
	if *created != 1 || len(records) != 0 || len(*journal) == 0 {
		t.Fatal("migration recreated or overwrote owner-managed data")
	}
}
func TestWireHushMigrationCrashAfterCreateResumesSameReservedIdentity(t *testing.T) {
	store, journal, records, created := testWireHushMigrationStore(t)
	write := store.writeJournal
	writes := 0
	store.writeJournal = func(data []byte) error {
		writes++
		if writes == 2 {
			return errors.New("interrupted completion")
		}
		return write(data)
	}
	source := migrationSource()
	if err := runWireHushMigration(context.Background(), []conf.LegacyMigrationSource{source}, store); err == nil {
		t.Fatal("interruption was hidden")
	}
	if len(*journal) == 0 || len(records) != 1 {
		t.Fatal("reserved identity or created record was lost")
	}
	store.writeJournal = write
	if err := runWireHushMigration(context.Background(), []conf.LegacyMigrationSource{source}, store); err != nil {
		t.Fatal(err)
	}
	if *created != 1 {
		t.Fatal("retry generated duplicate identity")
	}
}
func TestWireHushMigrationFailureNeverWritesBeforeBackupAndPlan(t *testing.T) {
	for _, stage := range []string{"backup", "journal", "verify"} {
		t.Run(stage, func(t *testing.T) {
			store, _, _, created := testWireHushMigrationStore(t)
			switch stage {
			case "backup":
				store.backup = func(conf.LegacyMigrationSource) error { return errors.New("backup failed") }
			case "journal":
				store.writeJournal = func([]byte) error { return errors.New("journal failed") }
			case "verify":
				store.load = func(conf.TunnelScope, string, conf.TunnelID) (conf.TunnelRecord, error) {
					return conf.TunnelRecord{}, errors.New("record verification failed")
				}
			}
			if err := runWireHushMigration(context.Background(), []conf.LegacyMigrationSource{migrationSource()}, store); err == nil {
				t.Fatal("migration failure was hidden")
			}
			if *created != 0 {
				t.Fatal("target was created without its prerequisites")
			}
		})
	}
}
func TestWireHushMigrationRejectsChangedOrMissingIncompleteSource(t *testing.T) {
	store, journal, _, _ := testWireHushMigrationStore(t)
	id, _ := conf.NewTunnelID()
	data, _ := json.Marshal(wireHushMigrationJournal{Version: 1, Entries: []wireHushMigrationEntry{{Product: "TunnelMint", FileName: "Home.conf.dpapi", TunnelID: id, Digest: "old"}}})
	*journal = data
	if err := runWireHushMigration(context.Background(), nil, store); err == nil {
		t.Fatal("missing unfinished source falsely completed")
	}
	if err := runWireHushMigration(context.Background(), []conf.LegacyMigrationSource{migrationSource()}, store); err == nil {
		t.Fatal("changed unfinished source was silently imported")
	}
}
func TestWireHushMigrationServiceOwnershipRequiresExactLegacyCommand(t *testing.T) {
	roots := map[string]string{"TunnelMint": `C:\Program Files\TunnelMint\Data`}
	valid := mgr.Config{ServiceType: windows.SERVICE_WIN32_OWN_PROCESS, ServiceStartName: "LocalSystem", BinaryPathName: `"C:\Program Files\TunnelMint\wirehush.exe" /tunnelservice "C:\Program Files\TunnelMint\Data\Configurations\Home.conf.dpapi"`}
	if !wireHushLegacyServiceOwned("TunnelMintTunnel$Home", valid, roots) {
		t.Fatal("valid legacy service not recognized")
	}
	for _, name := range []string{"WireGuardTunnel$Home", "TunnelMintTunnel$Different"} {
		if wireHushLegacyServiceOwned(name, valid, roots) {
			t.Fatal("foreign service recognized")
		}
	}
	for _, command := range []string{`"C:\Temp\wirehush.exe" /tunnelservice "C:\Program Files\TunnelMint\Data\Configurations\Home.conf.dpapi"`, `"C:\Program Files\TunnelMint\wirehush.exe" /tunnelservice C:\outside\Home.conf.dpapi`, valid.BinaryPathName + " extra", `"C:\Program Files\TunnelMint\wirehush.exe" /wirehushtunnelservice --id anything`} {
		config := valid
		config.BinaryPathName = command
		if wireHushLegacyServiceOwned("TunnelMintTunnel$Home", config, roots) {
			t.Fatal("unverified legacy command recognized")
		}
	}
}
