/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package conf

import (
	"os"
	"path/filepath"
	"testing"
)

func tunnelRecordListTestID(t *testing.T, text string) TunnelID {
	t.Helper()
	id, err := ParseTunnelID(text)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func tunnelRecordListRecord(t *testing.T, id string, scope TunnelScope, owner, name string) TunnelRecord {
	t.Helper()
	record := tunnelRecordStoreTestRecord(t, scope, owner, name)
	record.TunnelID = tunnelRecordListTestID(t, id)
	return record
}

func saveTunnelRecordListTestRecord(t *testing.T, root string, record TunnelRecord) {
	t.Helper()
	if err := saveTunnelRecordAtRoot(root, record, false, withTunnelRecordStoreTestSecurity(t)); err != nil {
		t.Fatal(err)
	}
}

func TestListTunnelRecordsAbsentNamespaces(t *testing.T) {
	for _, test := range []struct {
		name  string
		scope TunnelScope
		owner string
	}{{"private", TunnelScopePrivate, "S-1-5-18"}, {"shared", TunnelScopeShared, ""}} {
		t.Run(test.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "WireHush")
			records, err := listTunnelRecordsAtRoot(root, test.scope, test.owner)
			if err != nil || len(records) != 0 {
				t.Fatalf("records, err = %#v, %v", records, err)
			}
			if _, err := os.Stat(root); !os.IsNotExist(err) {
				t.Fatalf("listing created root: %v", err)
			}
		})
	}
}

func TestListTunnelRecordsNamespacesAndOrder(t *testing.T) {
	root := filepath.Join(t.TempDir(), "WireHush")
	privateA := tunnelRecordListRecord(t, "ffffffff-ffff-4fff-8fff-ffffffffffff", TunnelScopePrivate, "S-1-5-18", "Home")
	privateB := tunnelRecordListRecord(t, "11111111-1111-4111-8111-111111111111", TunnelScopePrivate, "S-1-5-18", "home")
	otherOwner := tunnelRecordListRecord(t, "22222222-2222-4222-8222-222222222222", TunnelScopePrivate, "S-1-5-19", "Other")
	sharedA := tunnelRecordListRecord(t, "33333333-3333-4333-8333-333333333333", TunnelScopeShared, "", "Shared")
	sharedB := tunnelRecordListRecord(t, "44444444-4444-4444-8444-444444444444", TunnelScopeShared, "", "Shared")
	for _, record := range []TunnelRecord{privateA, privateB, otherOwner, sharedB, sharedA} {
		saveTunnelRecordListTestRecord(t, root, record)
	}
	private, err := listTunnelRecordsAtRoot(root, TunnelScopePrivate, "S-1-5-18")
	if err != nil || len(private) != 2 || private[0].TunnelID != privateB.TunnelID || private[1].TunnelID != privateA.TunnelID {
		t.Fatalf("private records = %#v, %v", private, err)
	}
	shared, err := listTunnelRecordsAtRoot(root, TunnelScopeShared, "")
	if err != nil || len(shared) != 2 || shared[0].TunnelID != sharedA.TunnelID || shared[1].TunnelID != sharedB.TunnelID {
		t.Fatalf("shared records = %#v, %v", shared, err)
	}
	other, err := listTunnelRecordsAtRoot(root, TunnelScopePrivate, "S-1-5-19")
	if err != nil || len(other) != 1 || other[0].TunnelID != otherOwner.TunnelID {
		t.Fatalf("other owner records = %#v, %v", other, err)
	}
}

func TestListTunnelRecordsRejectsUnexpectedEntries(t *testing.T) {
	for _, name := range []string{
		"12345678-1234-4ABC-8def-1234567890ab.conf.dpapi", "Home.conf.dpapi", "not-an-id.conf.dpapi", "notes.txt", "short.tmp", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA.tmp", "gggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg.tmp",
	} {
		t.Run(name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "WireHush")
			sd := withTunnelRecordStoreTestSecurity(t)
			directory, err := ensureTunnelRecordDirectoryAtRoot(root, TunnelScopeShared, "", sd)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, name), []byte("unexpected"), 0o600); err != nil {
				t.Fatal(err)
			}
			if records, err := listTunnelRecordsAtRoot(root, TunnelScopeShared, ""); err == nil || records != nil {
				t.Fatalf("records, err = %#v, %v", records, err)
			}
		})
	}
	t.Run("unexpected subdirectory", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "WireHush")
		sd := withTunnelRecordStoreTestSecurity(t)
		directory, err := ensureTunnelRecordDirectoryAtRoot(root, TunnelScopeShared, "", sd)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(directory, "unexpected"), 0o700); err != nil {
			t.Fatal(err)
		}
		if records, err := listTunnelRecordsAtRoot(root, TunnelScopeShared, ""); err == nil || records != nil {
			t.Fatalf("records, err = %#v, %v", records, err)
		}
	})
}

func TestListTunnelRecordsIgnoresExactTemporaryFile(t *testing.T) {
	root := filepath.Join(t.TempDir(), "WireHush")
	record := tunnelRecordListRecord(t, "12345678-1234-4abc-8def-1234567890ab", TunnelScopeShared, "", "Home")
	saveTunnelRecordListTestRecord(t, root, record)
	directory, _ := tunnelRecordDirectoryFromRoot(root, TunnelScopeShared, "")
	if err := os.WriteFile(filepath.Join(directory, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef.tmp"), []byte("leftover"), 0o600); err != nil {
		t.Fatal(err)
	}
	records, err := listTunnelRecordsAtRoot(root, TunnelScopeShared, "")
	if err != nil || len(records) != 1 || records[0] != record {
		t.Fatalf("records, err = %#v, %v", records, err)
	}
}

func TestListTunnelRecordsFailsClosedForRecordsAndObjects(t *testing.T) {
	root := filepath.Join(t.TempDir(), "WireHush")
	valid := tunnelRecordListRecord(t, "12345678-1234-4abc-8def-1234567890ab", TunnelScopePrivate, "S-1-5-18", "Home")
	saveTunnelRecordListTestRecord(t, root, valid)
	sd := withTunnelRecordStoreTestSecurity(t)
	directory, _ := tunnelRecordDirectoryFromRoot(root, valid.Scope, valid.OwnerSID)
	otherID := tunnelRecordListTestID(t, "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee")
	if err := writeLockedDownFile(filepath.Join(directory, otherID.String()+tunnelRecordFileSuffix), false, []byte("corrupt")); err != nil {
		t.Fatal(err)
	}
	if records, err := listTunnelRecordsAtRoot(root, valid.Scope, valid.OwnerSID); err == nil || records != nil {
		t.Fatalf("partial records returned: %#v, %v", records, err)
	}
	if err := os.Remove(filepath.Join(directory, otherID.String()+tunnelRecordFileSuffix)); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(directory, otherID.String()+tunnelRecordFileSuffix), 0o700); err != nil {
		t.Fatal(err)
	}
	if records, err := listTunnelRecordsAtRoot(root, valid.Scope, valid.OwnerSID); err == nil || records != nil {
		t.Fatalf("directory record accepted: %#v, %v", records, err)
	}
	_ = sd
}

func TestListTunnelRecordsRejectsRelocatedRecords(t *testing.T) {
	root := filepath.Join(t.TempDir(), "WireHush")
	private := tunnelRecordListRecord(t, "12345678-1234-4abc-8def-1234567890ab", TunnelScopePrivate, "S-1-5-18", "Home")
	saveTunnelRecordListTestRecord(t, root, private)
	source, _ := tunnelRecordPathFromRoot(root, private.Scope, private.OwnerSID, private.TunnelID)
	raw, _ := os.ReadFile(source)
	sd := withTunnelRecordStoreTestSecurity(t)
	otherID := tunnelRecordListTestID(t, "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee")
	privateDirectory, _ := tunnelRecordDirectoryFromRoot(root, private.Scope, private.OwnerSID)
	if err := writeLockedDownFile(filepath.Join(privateDirectory, otherID.String()+tunnelRecordFileSuffix), false, raw); err != nil {
		t.Fatal(err)
	}
	if records, err := listTunnelRecordsAtRoot(root, TunnelScopePrivate, private.OwnerSID); err == nil || records != nil {
		t.Fatalf("wrong ID accepted: %#v, %v", records, err)
	}
	if err := os.Remove(filepath.Join(privateDirectory, otherID.String()+tunnelRecordFileSuffix)); err != nil {
		t.Fatal(err)
	}
	shared, err := ensureTunnelRecordDirectoryAtRoot(root, TunnelScopeShared, "", sd)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeLockedDownFile(filepath.Join(shared, private.TunnelID.String()+tunnelRecordFileSuffix), false, raw); err != nil {
		t.Fatal(err)
	}
	if records, err := listTunnelRecordsAtRoot(root, TunnelScopeShared, ""); err == nil || records != nil {
		t.Fatalf("wrong scope accepted: %#v, %v", records, err)
	}
	other, err := ensureTunnelRecordDirectoryAtRoot(root, TunnelScopePrivate, "S-1-5-19", sd)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeLockedDownFile(filepath.Join(other, private.TunnelID.String()+tunnelRecordFileSuffix), false, raw); err != nil {
		t.Fatal(err)
	}
	if records, err := listTunnelRecordsAtRoot(root, TunnelScopePrivate, "S-1-5-19"); err == nil || records != nil {
		t.Fatalf("wrong owner accepted: %#v, %v", records, err)
	}
}

func TestListTunnelRecordsRejectsInvalidIdentityAndHierarchy(t *testing.T) {
	root := filepath.Join(t.TempDir(), "WireHush")
	for _, test := range []struct {
		scope TunnelScope
		owner string
	}{{0, ""}, {TunnelScope(3), ""}, {TunnelScopePrivate, ""}, {TunnelScopePrivate, "not-a-sid"}, {TunnelScopePrivate, " S-1-5-18"}, {TunnelScopePrivate, "S-1-5-018"}, {TunnelScopeShared, "S-1-5-18"}} {
		if records, err := listTunnelRecordsAtRoot(root, test.scope, test.owner); err == nil || records != nil {
			t.Fatalf("invalid identity accepted: %#v, %v", records, err)
		}
	}
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, tunnelRecordConfigurationsDirectory), []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if records, err := listTunnelRecordsAtRoot(root, TunnelScopeShared, ""); err == nil || records != nil {
		t.Fatalf("invalid hierarchy accepted: %#v, %v", records, err)
	}
}
