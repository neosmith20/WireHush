/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package conf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"

	"golang.zx2c4.com/wireguard/windows/product"
)

func storagePathTestID(t *testing.T) TunnelID {
	t.Helper()
	id, err := ParseTunnelID("12345678-1234-4abc-8def-1234567890ab")
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestWireHushMachineDataRoot(t *testing.T) {
	programData, err := windows.KnownFolderPath(windows.FOLDERID_ProgramData, windows.KF_FLAG_DEFAULT)
	if err != nil {
		t.Fatal(err)
	}
	root, err := WireHushMachineDataRoot()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(programData, product.Name); root != want {
		t.Fatalf("WireHushMachineDataRoot() = %q, want %q", root, want)
	}
	if filepath.Base(root) != "WireHush" || filepath.Base(root) == "TunnelMint" {
		t.Fatalf("machine root final component = %q, want WireHush", filepath.Base(root))
	}
}

func TestTunnelRecordDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "does-not-exist")
	private, err := tunnelRecordDirectoryFromRoot(root, TunnelScopePrivate, "S-1-5-18")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "Configurations", "Users", "S-1-5-18"); private != want {
		t.Fatalf("private directory = %q, want %q", private, want)
	}
	shared, err := tunnelRecordDirectoryFromRoot(root, TunnelScopeShared, "")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "Configurations", "Shared"); shared != want {
		t.Fatalf("shared directory = %q, want %q", shared, want)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("pure derivation created root or returned unexpected error: %v", err)
	}
}

func TestTunnelRecordPath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "does-not-exist")
	id := storagePathTestID(t)
	private, err := tunnelRecordPathFromRoot(root, TunnelScopePrivate, "S-1-5-18", id)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "Configurations", "Users", "S-1-5-18", "12345678-1234-4abc-8def-1234567890ab.conf.dpapi"); private != want {
		t.Fatalf("private path = %q, want %q", private, want)
	}
	shared, err := tunnelRecordPathFromRoot(root, TunnelScopeShared, "", id)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "Configurations", "Shared", "12345678-1234-4abc-8def-1234567890ab.conf.dpapi"); shared != want {
		t.Fatalf("shared path = %q, want %q", shared, want)
	}
	if !strings.HasSuffix(shared, ".conf.dpapi") || !strings.Contains(filepath.Base(shared), id.String()) || strings.Contains(filepath.Base(shared), id.Token()) {
		t.Fatalf("record filename = %q, want canonical hyphenated ID with .conf.dpapi", filepath.Base(shared))
	}
}

func TestTunnelRecordPathRejectsInvalidInputs(t *testing.T) {
	root := filepath.Join(t.TempDir(), "does-not-exist")
	id := storagePathTestID(t)
	var zero TunnelID
	for _, test := range []struct {
		name     string
		scope    TunnelScope
		ownerSID string
		id       TunnelID
	}{
		{"zero ID", TunnelScopeShared, "", zero},
		{"zero scope", 0, "", id},
		{"out of range scope", TunnelScope(3), "", id},
		{"empty private owner", TunnelScopePrivate, "", id},
		{"malformed private owner", TunnelScopePrivate, "not-a-sid", id},
		{"leading whitespace private owner", TunnelScopePrivate, " S-1-5-18", id},
		{"trailing whitespace private owner", TunnelScopePrivate, "S-1-5-18 ", id},
		{"noncanonical private owner", TunnelScopePrivate, "S-1-5-018", id},
		{"traversal-like private owner", TunnelScopePrivate, "S-1-5-18\\..", id},
		{"shared nonempty owner", TunnelScopeShared, "S-1-5-18", id},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := tunnelRecordPathFromRoot(root, test.scope, test.ownerSID, test.id); err == nil {
				t.Fatal("invalid path input succeeded")
			}
		})
	}
}

func TestTunnelRecordPathDoesNotUseFriendlyName(t *testing.T) {
	root := filepath.Join(t.TempDir(), "does-not-exist")
	idA := storagePathTestID(t)
	idB, err := ParseTunnelID("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee")
	if err != nil {
		t.Fatal(err)
	}
	pathA, err := tunnelRecordPathFromRoot(root, TunnelScopePrivate, "S-1-5-18", idA)
	if err != nil {
		t.Fatal(err)
	}
	pathB, err := tunnelRecordPathFromRoot(root, TunnelScopePrivate, "S-1-5-18", idB)
	if err != nil {
		t.Fatal(err)
	}
	if pathA == pathB || strings.Contains(pathA, "Home") || strings.Contains(pathB, "Home") {
		t.Fatalf("record paths do not depend solely on distinct IDs: %q, %q", pathA, pathB)
	}
	otherOwner, err := tunnelRecordPathFromRoot(root, TunnelScopePrivate, "S-1-5-19", idA)
	if err != nil {
		t.Fatal(err)
	}
	if pathA == otherOwner {
		t.Fatal("different private owners produced the same path")
	}
}
