/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush contributors. All Rights Reserved.
 */

package tunnel

import (
	"encoding/binary"
	"testing"
	"unsafe"

	"golang.org/x/crypto/blake2s"
	"golang.org/x/sys/windows"
	"golang.org/x/text/unicode/norm"
	"golang.zx2c4.com/wireguard/windows/conf"
)

const (
	oldDeterministicGUIDLabel = "Deterministic TunnelMint Windows GUID v1"
	oldFixedGUIDLabel         = "Fixed TunnelMint Windows GUID v1"
)

func TestDeterministicGUIDUsesWireHushNamespace(t *testing.T) {
	if deterministicGUIDLabel != "Deterministic WireHush Windows GUID v1" {
		t.Fatalf("unexpected deterministic label: %q", deterministicGUIDLabel)
	}
	if fixedGUIDLabel != "Fixed WireHush Windows GUID v1" {
		t.Fatalf("unexpected fixed label: %q", fixedGUIDLabel)
	}
	if upstreamDeterministicGUIDLabel != "Deterministic WireGuard Windows GUID v1 jason@zx2c4.com" {
		t.Fatalf("unexpected upstream label: %q", upstreamDeterministicGUIDLabel)
	}
	if deterministicGUIDLabel == oldDeterministicGUIDLabel || fixedGUIDLabel == oldFixedGUIDLabel {
		t.Fatal("active GUID label retains the TunnelMint namespace")
	}

	c := &conf.Config{Name: "same-name", Interface: conf.Interface{PrivateKey: *conf.NewPrivateKey()}}
	wireHush := deterministicGUIDWithLabel(c, deterministicGUIDLabel)
	old := deterministicGUIDWithLabel(c, oldDeterministicGUIDLabel)
	upstream := deterministicGUIDWithLabel(c, upstreamDeterministicGUIDLabel)
	if *wireHush == *old {
		t.Fatal("WireHush deterministic GUID matches the old TunnelMint namespace")
	}
	if *wireHush == *upstream {
		t.Fatal("WireHush deterministic GUID collides with upstream namespace")
	}
	if *wireHush != *deterministicGUID(c) {
		t.Fatal("deterministicGUID did not use the WireHush namespace")
	}
}

func TestFixedGUIDUsesWireHushNamespace(t *testing.T) {
	c := &conf.Config{Name: "same-name"}
	previous := UseFixedGUIDInsteadOfDeterministic
	UseFixedGUIDInsteadOfDeterministic = true
	t.Cleanup(func() { UseFixedGUIDInsteadOfDeterministic = previous })

	if *deterministicGUID(c) == *fixedGUIDForLabel(c.Name, oldFixedGUIDLabel) {
		t.Fatal("fixed GUID matches the old TunnelMint namespace")
	}
	if *deterministicGUID(c) != *fixedGUIDForLabel(c.Name, fixedGUIDLabel) {
		t.Fatal("fixed GUID did not use the WireHush namespace")
	}
}

func fixedGUIDForLabel(name, label string) *windows.GUID {
	b2, _ := blake2s.New256(nil)
	b2.Write([]byte(label))
	nameBytes := norm.NFC.Bytes([]byte(name))
	var length [4]byte
	binary.LittleEndian.PutUint32(length[:], uint32(len(nameBytes)))
	b2.Write(length[:])
	b2.Write(nameBytes)
	return (*windows.GUID)(unsafe.Pointer(&b2.Sum(nil)[0]))
}
