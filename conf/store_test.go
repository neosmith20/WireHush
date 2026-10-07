/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2019-2026 WireGuard LLC. All Rights Reserved.
 */

package conf

import (
	"reflect"
	"slices"
	"sync/atomic"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestStorage(t *testing.T) {
	testRoot := t.TempDir()
	previousRootDir, previousConfigFileDir := cachedRootDir, cachedConfigFileDir
	previousEncryptedFileSd := atomic.LoadPointer(&encryptedFileSd)
	cachedRootDir, cachedConfigFileDir = "", ""
	PresetRootDirectory(testRoot)
	t.Cleanup(func() {
		cachedRootDir, cachedConfigFileDir = previousRootDir, previousConfigFileDir
		atomic.StorePointer(&encryptedFileSd, previousEncryptedFileSd)
	})
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		t.Fatalf("Unable to open current process token: %s", err.Error())
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		t.Fatalf("Unable to get current process user: %s", err.Error())
	}
	testFileSd, err := windows.SecurityDescriptorFromString("D:PAI(A;;FA;;;" + user.User.Sid.String() + ")")
	if err != nil {
		t.Fatalf("Unable to create test file security descriptor: %s", err.Error())
	}
	atomic.StorePointer(&encryptedFileSd, unsafe.Pointer(testFileSd))

	root, err := RootDirectory(true)
	if err != nil {
		t.Fatalf("Unable to resolve test root: %s", err.Error())
	}
	if root != testRoot {
		t.Fatalf("Storage root = %q, want temporary test root %q", root, testRoot)
	}

	c, err := FromWgQuick(testInput, "golangTest")
	if err != nil {
		t.Errorf("Unable to parse test config: %s", err.Error())
		return
	}

	err = c.Save(true)
	if err != nil {
		t.Errorf("Unable to save config: %s", err.Error())
	}

	configs, err := ListConfigNames()
	if err != nil {
		t.Errorf("Unable to list configs: %s", err.Error())
	}

	found := slices.Contains(configs, "golangTest")
	if !found {
		t.Error("Unable to find saved config in list")
	}

	loaded, err := LoadFromName("golangTest")
	if err != nil {
		t.Errorf("Unable to load config: %s", err.Error())
		return
	}

	if !reflect.DeepEqual(loaded, c) {
		t.Error("Loaded config is not the same as saved config")
	}

	k := NewPrivateKey()
	c.Interface.PrivateKey = *k

	err = c.Save(false)
	if err == nil {
		t.Error("Config disappeared or was unexpectedly overwritten")
	}
	err = c.Save(true)
	if err != nil {
		t.Errorf("Unable to save config a second time: %s", err.Error())
	}

	loaded, err = LoadFromName("golangTest")
	if err != nil {
		t.Errorf("Unable to load config a second time: %s", err.Error())
		return
	}

	if !reflect.DeepEqual(loaded, c) {
		t.Error("Second loaded config is not the same as second saved config")
	}

	err = DeleteName("golangTest")
	if err != nil {
		t.Errorf("Unable to delete config: %s", err.Error())
	}

	configs, err = ListConfigNames()
	if err != nil {
		t.Errorf("Unable to list configs: %s", err.Error())
	}
	found = slices.Contains(configs, "golangTest")
	if found {
		t.Error("Config wasn't actually deleted")
	}
}
