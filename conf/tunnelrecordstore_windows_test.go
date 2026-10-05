/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package conf

import (
	"bytes"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

const tunnelRecordStoreTestWGQuick = "# preserve this comment\n[Interface]\nPrivateKey = AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\nAddress = 10.0.0.2/32\nPostUp = echo retained-script\n\n[Peer]\nPublicKey = BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB=\nAllowedIPs = 0.0.0.0/0\n"

func tunnelRecordStoreTestSecurityDescriptor(t *testing.T) *windows.SECURITY_DESCRIPTOR {
	t.Helper()
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		t.Fatal(err)
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	userSID := user.User.Sid.String()
	sd, err := windows.SecurityDescriptorFromString("O:" + userSID + "G:" + userSID + "D:PAI(A;;FA;;;" + userSID + ")")
	if err != nil {
		t.Fatal(err)
	}
	return sd
}

func withTunnelRecordStoreTestSecurity(t *testing.T) *windows.SECURITY_DESCRIPTOR {
	t.Helper()
	sd := tunnelRecordStoreTestSecurityDescriptor(t)
	previous := atomic.LoadPointer(&encryptedFileSd)
	atomic.StorePointer(&encryptedFileSd, unsafe.Pointer(sd))
	t.Cleanup(func() { atomic.StorePointer(&encryptedFileSd, previous) })
	return sd
}

func tunnelRecordStoreTestRecord(t *testing.T, scope TunnelScope, owner string, name string) TunnelRecord {
	t.Helper()
	return TunnelRecord{TunnelID: storagePathTestID(t), Name: name, Scope: scope, OwnerSID: owner, WGQuickText: tunnelRecordStoreTestWGQuick}
}

func TestTunnelRecordStorePrivateAndSharedRoundTrip(t *testing.T) {
	sd := withTunnelRecordStoreTestSecurity(t)
	for _, test := range []struct {
		name  string
		scope TunnelScope
		owner string
	}{{"private", TunnelScopePrivate, "S-1-5-18"}, {"shared", TunnelScopeShared, ""}} {
		t.Run(test.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "WireHush")
			record := tunnelRecordStoreTestRecord(t, test.scope, test.owner, "Home")
			if err := saveTunnelRecordAtRoot(root, record, false, sd); err != nil {
				t.Fatal(err)
			}
			got, err := loadTunnelRecordAtRoot(root, test.scope, test.owner, record.TunnelID)
			if err != nil {
				t.Fatal(err)
			}
			if got != record {
				t.Fatalf("record = %#v, want %#v", got, record)
			}
			path, _ := tunnelRecordPathFromRoot(root, test.scope, test.owner, record.TunnelID)
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			plain, _ := MarshalTunnelRecord(record)
			if bytes.Equal(raw, plain) {
				t.Fatal("record file equals its plaintext JSON")
			}
			for _, plaintext := range [][]byte{
				[]byte(record.WGQuickText),
				[]byte("PrivateKey = AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="),
				[]byte("# preserve this comment"),
				[]byte("PostUp = echo retained-script"),
				[]byte(`"wg_quick_text"`),
			} {
				if bytes.Contains(raw, plaintext) {
					t.Fatalf("record ciphertext contains plaintext %q", plaintext)
				}
			}
		})
	}
}

func TestTunnelRecordStoreDescriptionAndOverwrite(t *testing.T) {
	sd := withTunnelRecordStoreTestSecurity(t)
	root := filepath.Join(t.TempDir(), "WireHush")
	record := tunnelRecordStoreTestRecord(t, TunnelScopePrivate, "S-1-5-18", "Home")
	if got, want := tunnelRecordDPAPIDescription(record.TunnelID), "WireHush Tunnel 12345678-1234-4abc-8def-1234567890ab"; got != want {
		t.Fatalf("description = %q, want %q", got, want)
	}
	if err := saveTunnelRecordAtRoot(root, record, false, sd); err != nil {
		t.Fatal(err)
	}
	path, _ := tunnelRecordPathFromRoot(root, record.Scope, record.OwnerSID, record.TunnelID)
	updated := record
	updated.Name = "Office"
	if tunnelRecordDPAPIDescription(updated.TunnelID) != tunnelRecordDPAPIDescription(record.TunnelID) {
		t.Fatal("name changed DPAPI description")
	}
	if err := saveTunnelRecordAtRoot(root, updated, false, sd); err == nil {
		t.Fatal("overwrite=false replaced existing record")
	}
	got, err := loadTunnelRecordAtRoot(root, record.Scope, record.OwnerSID, record.TunnelID)
	if err != nil || got.Name != "Home" {
		t.Fatalf("original record not preserved: %#v, %v", got, err)
	}
	if err := saveTunnelRecordAtRoot(root, updated, true, sd); err != nil {
		t.Fatal(err)
	}
	path2, _ := tunnelRecordPathFromRoot(root, updated.Scope, updated.OwnerSID, updated.TunnelID)
	if path != path2 {
		t.Fatal("rename changed record path")
	}
	got, err = loadTunnelRecordAtRoot(root, updated.Scope, updated.OwnerSID, updated.TunnelID)
	if err != nil || got.Name != "Office" {
		t.Fatalf("updated record not loaded: %#v, %v", got, err)
	}
}

func TestTunnelRecordStoreRejectsRelocatedAndCorruptCiphertext(t *testing.T) {
	sd := withTunnelRecordStoreTestSecurity(t)
	root := filepath.Join(t.TempDir(), "WireHush")
	record := tunnelRecordStoreTestRecord(t, TunnelScopePrivate, "S-1-5-18", "Home")
	if err := saveTunnelRecordAtRoot(root, record, false, sd); err != nil {
		t.Fatal(err)
	}
	source, _ := tunnelRecordPathFromRoot(root, record.Scope, record.OwnerSID, record.TunnelID)
	raw, _ := os.ReadFile(source)
	otherID, _ := ParseTunnelID("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee")
	otherPath, _ := tunnelRecordPathFromRoot(root, record.Scope, record.OwnerSID, otherID)
	if err := writeLockedDownFile(otherPath, false, raw); err != nil {
		t.Fatal(err)
	}
	if _, err := loadTunnelRecordAtRoot(root, record.Scope, record.OwnerSID, otherID); err == nil {
		t.Fatal("wrong-ID ciphertext loaded")
	}
	shared, err := ensureTunnelRecordDirectoryAtRoot(root, TunnelScopeShared, "", sd)
	if err != nil {
		t.Fatal(err)
	}
	sharedPath := filepath.Join(shared, record.TunnelID.String()+tunnelRecordFileSuffix)
	if err := writeLockedDownFile(sharedPath, false, raw); err != nil {
		t.Fatal(err)
	}
	if _, err := loadTunnelRecordAtRoot(root, TunnelScopeShared, "", record.TunnelID); err == nil {
		t.Fatal("wrong-scope ciphertext loaded")
	}
	otherOwner, err := ensureTunnelRecordDirectoryAtRoot(root, TunnelScopePrivate, "S-1-5-19", sd)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeLockedDownFile(filepath.Join(otherOwner, record.TunnelID.String()+tunnelRecordFileSuffix), false, raw); err != nil {
		t.Fatal(err)
	}
	if _, err := loadTunnelRecordAtRoot(root, TunnelScopePrivate, "S-1-5-19", record.TunnelID); err == nil {
		t.Fatal("wrong-owner ciphertext loaded")
	}
	if err := os.WriteFile(source, []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadTunnelRecordAtRoot(root, record.Scope, record.OwnerSID, record.TunnelID); err == nil {
		t.Fatal("corrupt ciphertext loaded")
	}
}

func TestTunnelRecordStoreDeleteAndInvalidInputs(t *testing.T) {
	sd := withTunnelRecordStoreTestSecurity(t)
	root := filepath.Join(t.TempDir(), "WireHush")
	record := tunnelRecordStoreTestRecord(t, TunnelScopePrivate, "S-1-5-18", "Home")
	if err := saveTunnelRecordAtRoot(root, record, false, sd); err != nil {
		t.Fatal(err)
	}
	path, _ := tunnelRecordPathFromRoot(root, record.Scope, record.OwnerSID, record.TunnelID)
	parent := filepath.Dir(path)
	if err := deleteTunnelRecordAtRoot(root, record.Scope, record.OwnerSID, record.TunnelID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("record still exists")
	}
	if _, err := os.Stat(parent); err != nil {
		t.Fatal("parent removed")
	}
	if _, err := loadTunnelRecordAtRoot(root, record.Scope, record.OwnerSID, record.TunnelID); err == nil {
		t.Fatal("deleted record loaded")
	}
	for _, invalid := range []TunnelRecord{
		func() TunnelRecord { r := record; r.TunnelID = TunnelID{}; return r }(),
		func() TunnelRecord { r := record; r.Scope = 0; return r }(),
		func() TunnelRecord { r := record; r.OwnerSID = ""; return r }(),
		func() TunnelRecord { r := record; r.OwnerSID = "not-a-sid"; return r }(),
		func() TunnelRecord { r := record; r.OwnerSID = "S-1-5-018"; return r }(),
		func() TunnelRecord { r := record; r.WGQuickText = "bad"; return r }(),
		func() TunnelRecord { r := record; r.Scope = TunnelScopeShared; r.OwnerSID = "S-1-5-18"; return r }(),
	} {
		invalidRoot := filepath.Join(t.TempDir(), "none")
		if err := saveTunnelRecordAtRoot(invalidRoot, invalid, false, sd); err == nil {
			t.Fatalf("invalid record saved: %#v", invalid)
		}
		if _, err := os.Stat(invalidRoot); !os.IsNotExist(err) {
			t.Fatalf("invalid record created storage root: %v", err)
		}
	}
}

func TestTunnelRecordStoreRejectsInvalidDirectoryComponent(t *testing.T) {
	sd := withTunnelRecordStoreTestSecurity(t)
	root := filepath.Join(t.TempDir(), "WireHush")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, tunnelRecordConfigurationsDirectory), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	record := tunnelRecordStoreTestRecord(t, TunnelScopeShared, "", "Home")
	if err := saveTunnelRecordAtRoot(root, record, false, sd); err == nil {
		t.Fatal("save accepted file as a directory component")
	}
}

func TestTunnelRecordStoreRejectsDirectoryAtRecordPath(t *testing.T) {
	sd := withTunnelRecordStoreTestSecurity(t)
	root := filepath.Join(t.TempDir(), "WireHush")
	record := tunnelRecordStoreTestRecord(t, TunnelScopeShared, "", "Home")
	dir, err := ensureTunnelRecordDirectoryAtRoot(root, record.Scope, record.OwnerSID, sd)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, record.TunnelID.String()+tunnelRecordFileSuffix)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := loadTunnelRecordAtRoot(root, record.Scope, record.OwnerSID, record.TunnelID); err == nil {
		t.Fatal("directory record path loaded")
	}
	if err := deleteTunnelRecordAtRoot(root, record.Scope, record.OwnerSID, record.TunnelID); err == nil {
		t.Fatal("directory record path deleted")
	}
}
