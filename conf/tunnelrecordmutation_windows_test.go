/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package conf

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"golang.org/x/sys/windows"
)

func saveTunnelRecordMutationTest(t *testing.T, root string, record TunnelRecord, overwrite bool, sd *windows.SECURITY_DESCRIPTOR) error {
	t.Helper()
	return saveTunnelRecordMutationAtRoot(root, record, overwrite, sd)
}

func TestTunnelRecordMutationNameConflictsAndRenames(t *testing.T) {
	root := filepath.Join(t.TempDir(), "WireHush")
	sd := withTunnelRecordStoreTestSecurity(t)
	a := tunnelRecordListRecord(t, "11111111-1111-4111-8111-111111111111", TunnelScopePrivate, "S-1-5-18", "Home")
	b := tunnelRecordListRecord(t, "22222222-2222-4222-8222-222222222222", TunnelScopePrivate, "S-1-5-18", "Other")
	if err := saveTunnelRecordMutationTest(t, root, a, false, sd); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Home", "home", "HOME"} {
		candidate := b
		candidate.Name = name
		if err := saveTunnelRecordMutationTest(t, root, candidate, false, sd); !errors.Is(err, ErrTunnelNameInUse) {
			t.Fatalf("save %q error = %v, want ErrTunnelNameInUse", name, err)
		}
	}
	if _, err := loadTunnelRecordAtRoot(root, a.Scope, a.OwnerSID, b.TunnelID); err == nil {
		t.Fatal("conflicting record was written")
	}
	loaded, err := loadTunnelRecordAtRoot(root, a.Scope, a.OwnerSID, a.TunnelID)
	if err != nil || loaded.Name != "Home" {
		t.Fatalf("original changed: %#v, %v", loaded, err)
	}
	path, _ := tunnelRecordPathFromRoot(root, a.Scope, a.OwnerSID, a.TunnelID)
	renamed := a
	renamed.Name = "Office"
	if err := saveTunnelRecordMutationTest(t, root, renamed, true, sd); err != nil {
		t.Fatal(err)
	}
	pathAfter, _ := tunnelRecordPathFromRoot(root, renamed.Scope, renamed.OwnerSID, renamed.TunnelID)
	if path != pathAfter || tunnelRecordDPAPIDescription(a.TunnelID) != tunnelRecordDPAPIDescription(renamed.TunnelID) {
		t.Fatal("rename changed identity")
	}
	renamed.Name = "office"
	if err := saveTunnelRecordMutationTest(t, root, renamed, true, sd); err != nil {
		t.Fatal(err)
	}
	loaded, err = loadTunnelRecordAtRoot(root, a.Scope, a.OwnerSID, a.TunnelID)
	if err != nil || loaded.Name != "office" {
		t.Fatalf("case-only rename failed: %#v, %v", loaded, err)
	}
	b.Name = "Office"
	if err := saveTunnelRecordMutationTest(t, root, b, false, sd); !errors.Is(err, ErrTunnelNameInUse) {
		t.Fatalf("rename conflict = %v", err)
	}
}

func TestTunnelRecordMutationNamespaceScopeAndBrokenState(t *testing.T) {
	root := filepath.Join(t.TempDir(), "WireHush")
	sd := withTunnelRecordStoreTestSecurity(t)
	a := tunnelRecordListRecord(t, "11111111-1111-4111-8111-111111111111", TunnelScopePrivate, "S-1-5-18", "Home")
	b := tunnelRecordListRecord(t, "22222222-2222-4222-8222-222222222222", TunnelScopePrivate, "S-1-5-19", "home")
	shared := tunnelRecordListRecord(t, "33333333-3333-4333-8333-333333333333", TunnelScopeShared, "", "HOME")
	for _, record := range []TunnelRecord{a, b, shared} {
		if err := saveTunnelRecordMutationTest(t, root, record, false, sd); err != nil {
			t.Fatal(err)
		}
	}
	brokenA := tunnelRecordListRecord(t, "44444444-4444-4444-8444-444444444444", TunnelScopePrivate, "S-1-5-21", "Home")
	brokenB := tunnelRecordListRecord(t, "55555555-5555-4555-8555-555555555555", TunnelScopePrivate, "S-1-5-21", "home")
	for _, record := range []TunnelRecord{brokenA, brokenB} {
		if err := saveTunnelRecordAtRoot(root, record, false, sd); err != nil {
			t.Fatal(err)
		}
	}
	repair := brokenA
	repair.Name = "Office"
	if err := saveTunnelRecordMutationTest(t, root, repair, true, sd); err != nil {
		t.Fatalf("candidate-involved repair failed: %v", err)
	}
	work := tunnelRecordListRecord(t, "66666666-6666-4666-8666-666666666666", TunnelScopePrivate, "S-1-5-22", "Work")
	home := tunnelRecordListRecord(t, "77777777-7777-4777-8777-777777777777", TunnelScopePrivate, "S-1-5-22", "Home")
	home2 := tunnelRecordListRecord(t, "88888888-8888-4888-8888-888888888888", TunnelScopePrivate, "S-1-5-22", "home")
	for _, record := range []TunnelRecord{work, home, home2} {
		if err := saveTunnelRecordAtRoot(root, record, false, sd); err != nil {
			t.Fatal(err)
		}
	}
	updated := work
	updated.Name = "Office"
	if err := saveTunnelRecordMutationTest(t, root, updated, true, sd); !errors.Is(err, ErrTunnelNamespaceNameConflict) {
		t.Fatalf("unrelated duplicate error = %v", err)
	}
	loaded, err := loadTunnelRecordAtRoot(root, work.Scope, work.OwnerSID, work.TunnelID)
	if err != nil || loaded.Name != "Work" {
		t.Fatalf("blocked save mutated work: %#v, %v", loaded, err)
	}
	if err := deleteTunnelRecordMutationAtRoot(root, brokenB.Scope, brokenB.OwnerSID, brokenB.TunnelID); err != nil {
		t.Fatal(err)
	}
	listed, err := listTunnelRecordsAtRoot(root, brokenA.Scope, brokenA.OwnerSID)
	if err != nil || len(listed) != 1 || listed[0].Name != "Office" {
		t.Fatalf("delete repair = %#v, %v", listed, err)
	}
}

func TestTunnelRecordMutationConcurrentSaves(t *testing.T) {
	for _, test := range []struct {
		name, secondName string
		wantSuccess      int
	}{{"same name", "home", 1}, {"distinct names", "Office", 2}} {
		t.Run(test.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "WireHush")
			sd := withTunnelRecordStoreTestSecurity(t)
			a := tunnelRecordListRecord(t, "11111111-1111-4111-8111-111111111111", TunnelScopePrivate, "S-1-5-18", "Home")
			b := tunnelRecordListRecord(t, "22222222-2222-4222-8222-222222222222", TunnelScopePrivate, "S-1-5-18", test.secondName)
			start := make(chan struct{})
			results := make(chan error, 2)
			var ready sync.WaitGroup
			ready.Add(2)
			for _, record := range []TunnelRecord{a, b} {
				go func(record TunnelRecord) {
					ready.Done()
					<-start
					results <- saveTunnelRecordMutationAtRoot(root, record, false, sd)
				}(record)
			}
			ready.Wait()
			close(start)
			first, second := <-results, <-results
			successes := 0
			for _, err := range []error{first, second} {
				if err == nil {
					successes++
				} else if test.wantSuccess == 1 && !errors.Is(err, ErrTunnelNameInUse) {
					t.Fatalf("unexpected race error: %v", err)
				}
			}
			if successes != test.wantSuccess {
				t.Fatalf("successes = %d, errors = %v, %v", successes, first, second)
			}
			listed, err := listTunnelRecordsAtRoot(root, a.Scope, a.OwnerSID)
			if err != nil || len(listed) != test.wantSuccess {
				t.Fatalf("final records = %#v, %v", listed, err)
			}
		})
	}
}

func TestTunnelRecordMutationDeleteSaveRace(t *testing.T) {
	root := filepath.Join(t.TempDir(), "WireHush")
	sd := withTunnelRecordStoreTestSecurity(t)
	a := tunnelRecordListRecord(t, "11111111-1111-4111-8111-111111111111", TunnelScopePrivate, "S-1-5-18", "Home")
	b := tunnelRecordListRecord(t, "22222222-2222-4222-8222-222222222222", TunnelScopePrivate, "S-1-5-18", "Home")
	if err := saveTunnelRecordMutationAtRoot(root, a, false, sd); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	var ready sync.WaitGroup
	ready.Add(2)
	deleteResult := make(chan error, 1)
	saveResult := make(chan error, 1)
	go func() {
		ready.Done()
		<-start
		deleteResult <- deleteTunnelRecordMutationAtRoot(root, a.Scope, a.OwnerSID, a.TunnelID)
	}()
	go func() { ready.Done(); <-start; saveResult <- saveTunnelRecordMutationAtRoot(root, b, false, sd) }()
	ready.Wait()
	close(start)
	if err := <-deleteResult; err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if err := <-saveResult; err != nil && !errors.Is(err, ErrTunnelNameInUse) {
		t.Fatalf("save error: %v", err)
	}
	listed, err := listTunnelRecordsAtRoot(root, a.Scope, a.OwnerSID)
	if err != nil || len(listed) > 1 {
		t.Fatalf("final records = %#v, %v", listed, err)
	}
}

func TestTunnelRecordMutationCorruptNamespaceBlocksSave(t *testing.T) {
	root := filepath.Join(t.TempDir(), "WireHush")
	sd := withTunnelRecordStoreTestSecurity(t)
	corruptID := tunnelRecordListTestID(t, "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee")
	directory, err := ensureTunnelRecordDirectoryAtRoot(root, TunnelScopeShared, "", sd)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeLockedDownFile(filepath.Join(directory, corruptID.String()+tunnelRecordFileSuffix), false, []byte("corrupt")); err != nil {
		t.Fatal(err)
	}
	candidate := tunnelRecordListRecord(t, "11111111-1111-4111-8111-111111111111", TunnelScopeShared, "", "Home")
	if err := saveTunnelRecordMutationAtRoot(root, candidate, false, sd); err == nil {
		t.Fatal("save accepted corrupt namespace")
	}
	if _, err := loadTunnelRecordAtRoot(root, candidate.Scope, candidate.OwnerSID, candidate.TunnelID); err == nil {
		t.Fatal("blocked save created candidate record")
	}
}

func TestTunnelRecordMutationExistingDuplicatePrecedesCandidateConflict(t *testing.T) {
	root := filepath.Join(t.TempDir(), "WireHush")
	sd := withTunnelRecordStoreTestSecurity(t)
	office := tunnelRecordListRecord(t, "22222222-2222-4222-8222-222222222222", TunnelScopePrivate, "S-1-5-18", "Office")
	home := tunnelRecordListRecord(t, "33333333-3333-4333-8333-333333333333", TunnelScopePrivate, "S-1-5-18", "Home")
	homeLower := tunnelRecordListRecord(t, "44444444-4444-4444-8444-444444444444", TunnelScopePrivate, "S-1-5-18", "home")
	for _, record := range []TunnelRecord{office, home, homeLower} {
		if err := saveTunnelRecordAtRoot(root, record, false, sd); err != nil {
			t.Fatal(err)
		}
	}
	candidate := tunnelRecordListRecord(t, "11111111-1111-4111-8111-111111111111", TunnelScopePrivate, "S-1-5-18", "Office")
	if err := saveTunnelRecordMutationAtRoot(root, candidate, false, sd); !errors.Is(err, ErrTunnelNamespaceNameConflict) || errors.Is(err, ErrTunnelNameInUse) {
		t.Fatalf("error = %v, want ErrTunnelNamespaceNameConflict", err)
	}
	if _, err := loadTunnelRecordAtRoot(root, candidate.Scope, candidate.OwnerSID, candidate.TunnelID); err == nil {
		t.Fatal("candidate was created")
	}
	for _, record := range []TunnelRecord{office, home, homeLower} {
		loaded, err := loadTunnelRecordAtRoot(root, record.Scope, record.OwnerSID, record.TunnelID)
		if err != nil || loaded.Name != record.Name {
			t.Fatalf("record changed: %#v, %v", loaded, err)
		}
	}
}

func TestTunnelRecordMutationConflictingRenamePreservesBothRecords(t *testing.T) {
	root := filepath.Join(t.TempDir(), "WireHush")
	sd := withTunnelRecordStoreTestSecurity(t)
	a := tunnelRecordListRecord(t, "11111111-1111-4111-8111-111111111111", TunnelScopePrivate, "S-1-5-18", "Home")
	b := tunnelRecordListRecord(t, "22222222-2222-4222-8222-222222222222", TunnelScopePrivate, "S-1-5-18", "Office")
	for _, record := range []TunnelRecord{a, b} {
		if err := saveTunnelRecordMutationAtRoot(root, record, false, sd); err != nil {
			t.Fatal(err)
		}
	}
	candidate := a
	candidate.Name = "office"
	if err := saveTunnelRecordMutationAtRoot(root, candidate, true, sd); !errors.Is(err, ErrTunnelNameInUse) {
		t.Fatalf("rename error = %v", err)
	}
	for _, record := range []TunnelRecord{a, b} {
		loaded, err := loadTunnelRecordAtRoot(root, record.Scope, record.OwnerSID, record.TunnelID)
		if err != nil || loaded.Name != record.Name {
			t.Fatalf("record changed: %#v, %v", loaded, err)
		}
	}
	listed, err := listTunnelRecordsAtRoot(root, a.Scope, a.OwnerSID)
	if err != nil || len(listed) != 2 {
		t.Fatalf("namespace = %#v, %v", listed, err)
	}
}

func TestTunnelRecordMutationExportedSaveRejectsInvalidRecord(t *testing.T) {
	if err := SaveTunnelRecord(TunnelRecord{}, false); err == nil {
		t.Fatal("exported SaveTunnelRecord accepted a zero TunnelID")
	}
}
