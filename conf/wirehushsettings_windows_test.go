/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */
package conf

import (
	"bytes"
	"golang.zx2c4.com/wireguard/windows/bootstrap"
	"golang.zx2c4.com/wireguard/windows/conf/dpapi"
	"os"
	"path/filepath"
	"testing"
)

func TestWireHushSettingsEncryptValidateAndPreserveOnFailure(t *testing.T) {
	withTunnelRecordStoreTestSecurity(t)
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "Settings"), 0700); err != nil {
		t.Fatal(err)
	}
	defaults, err := loadWireHushBootstrapAtRoot(root)
	if err != nil || len(defaults.Entries) == 0 {
		t.Fatal("missing settings did not retain built-in defaults")
	}
	settings := bootstrap.DefaultSettings()
	settings.Entries[0].Enabled = false
	if err := saveWireHushBootstrapAtRoot(root, settings); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "Settings", "bootstrap-dns.dpapi")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(settings.Entries[0].Address.String())) || bytes.Contains(raw, []byte("enabled")) {
		t.Fatal("settings stored in plaintext")
	}
	if _, err := dpapi.Decrypt(raw, "incorrect description"); err == nil {
		t.Fatal("settings accepted a different DPAPI identity")
	}
	got, err := loadWireHushBootstrapAtRoot(root)
	if err != nil || got.Entries[0].Enabled {
		t.Fatal("settings did not round-trip")
	}
	if err := saveWireHushBootstrapAtRoot(root, bootstrap.Settings{}); err == nil {
		t.Fatal("invalid settings overwrote protected state")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(raw, after) {
		t.Fatal("failed settings update changed ciphertext")
	}
	if err := os.WriteFile(path, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadWireHushBootstrapAtRoot(root); err == nil {
		t.Fatal("corrupt settings silently fell back instead of failing closed")
	}
}
func TestWireHushLogRejectsHardLinkBeforeModifyingTarget(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "Logs"), 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "outside.txt")
	sentinel := []byte("must remain unchanged")
	if err := os.WriteFile(target, sentinel, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(target, filepath.Join(root, "Logs", "backend.bin")); err != nil {
		t.Fatal(err)
	}
	if file, err := openWireHushLogFileAtRoot(root, tunnelRecordStoreTestSecurityDescriptor(t)); err == nil {
		file.Close()
		t.Fatal("hard-linked log file accepted")
	}
	actual, _ := os.ReadFile(target)
	if !bytes.Equal(actual, sentinel) {
		t.Fatal("log setup modified an unrelated hard-link target")
	}
}
