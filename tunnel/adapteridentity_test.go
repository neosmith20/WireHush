/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package tunnel

import (
	"encoding/binary"
	"encoding/hex"
	"testing"

	"golang.org/x/crypto/blake2s"
	"golang.org/x/sys/windows"

	"golang.zx2c4.com/wireguard/windows/conf"
	"golang.zx2c4.com/wireguard/windows/driver"
)

func adapterIdentityTestID(t *testing.T, text string) conf.TunnelID {
	t.Helper()
	id, err := conf.ParseTunnelID(text)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func adapterGUIDForTest(label, text string) [32]byte {
	var length [4]byte
	binary.LittleEndian.PutUint32(length[:], uint32(len(text)))
	return blake2s.Sum256(append(append([]byte(label), length[:]...), text...))
}

func adapterGUIDFromDigest(digest [32]byte) windows.GUID {
	return windows.GUID{
		Data1: binary.LittleEndian.Uint32(digest[0:4]),
		Data2: binary.LittleEndian.Uint16(digest[4:6]),
		Data3: binary.LittleEndian.Uint16(digest[6:8]),
		Data4: [8]byte(digest[8:16]),
	}
}

func TestAdapterIdentityReferenceVector(t *testing.T) {
	id := adapterIdentityTestID(t, "12345678-1234-4abc-8def-1234567890ab")
	identity, err := wireHushAdapterIdentityForTunnelID(id)
	if err != nil {
		t.Fatal(err)
	}
	if identity.Name != "WireHush-1234567812344abc8def1234567890ab" {
		t.Fatalf("name = %q", identity.Name)
	}
	if len(identity.Name) != 41 || len(identity.Name) >= driver.AdapterNameMax {
		t.Fatalf("adapter name length = %d", len(identity.Name))
	}
	for _, character := range identity.Name {
		if !(character >= 'a' && character <= 'z') && !(character >= 'A' && character <= 'Z') && !(character >= '0' && character <= '9') && character != '-' {
			t.Fatalf("invalid adapter name character %q", character)
		}
	}
	digest := adapterGUIDForTest(deterministicGUIDLabel, id.String())
	if got := hex.EncodeToString(digest[:]); got != "b9540945d7c639de808980e4307c644102951116f226026db095a7b823de3843" {
		t.Fatalf("digest = %s", got)
	}
	if identity.GUID.Data1 != 0x450954b9 || identity.GUID.Data2 != 0xc6d7 || identity.GUID.Data3 != 0xde39 || identity.GUID.Data4 != [8]byte{0x80, 0x89, 0x80, 0xe4, 0x30, 0x7c, 0x64, 0x41} {
		t.Fatalf("unexpected GUID fields: %#v", identity.GUID)
	}
}

func TestAdapterIdentityValidationAndSeparation(t *testing.T) {
	if _, err := wireHushAdapterIdentityForTunnelID(conf.TunnelID{}); err == nil {
		t.Fatal("zero TunnelID accepted")
	}
	a := adapterIdentityTestID(t, "12345678-1234-4abc-8def-1234567890ab")
	b := adapterIdentityTestID(t, "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee")
	first, err := wireHushAdapterIdentityForTunnelID(a)
	if err != nil {
		t.Fatal(err)
	}
	second, err := wireHushAdapterIdentityForTunnelID(a)
	if err != nil {
		t.Fatal(err)
	}
	other, err := wireHushAdapterIdentityForTunnelID(b)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("same TunnelID was not deterministic")
	}
	if first.Name == other.Name || first.GUID == other.GUID {
		t.Fatal("different TunnelIDs share adapter identity")
	}
}

func TestAdapterIdentityUsesStringLabelAndIgnoresFixedGUID(t *testing.T) {
	id := adapterIdentityTestID(t, "12345678-1234-4abc-8def-1234567890ab")
	previous := UseFixedGUIDInsteadOfDeterministic
	t.Cleanup(func() { UseFixedGUIDInsteadOfDeterministic = previous })
	UseFixedGUIDInsteadOfDeterministic = false
	standard, err := wireHushAdapterIdentityForTunnelID(id)
	if err != nil {
		t.Fatal(err)
	}
	UseFixedGUIDInsteadOfDeterministic = true
	fixed, err := wireHushAdapterIdentityForTunnelID(id)
	if err != nil {
		t.Fatal(err)
	}
	if standard != fixed {
		t.Fatal("fixed GUID escape hatch changed TunnelID identity")
	}
	if want := adapterGUIDFromDigest(adapterGUIDForTest(deterministicGUIDLabel, id.String())); standard.GUID != want {
		t.Fatal("GUID did not use canonical TunnelID string")
	}
	if token := adapterGUIDFromDigest(adapterGUIDForTest(deterministicGUIDLabel, id.Token())); standard.GUID == token {
		t.Fatal("GUID used TunnelID token")
	}
	for _, label := range []string{"Deterministic TunnelMint Windows GUID v1", upstreamDeterministicGUIDLabel} {
		other := adapterGUIDFromDigest(adapterGUIDForTest(label, id.String()))
		if standard.GUID == other {
			t.Fatal("GUID used wrong label")
		}
	}
}

func TestAdapterIdentityDependsOnlyOnTunnelID(t *testing.T) {
	id := adapterIdentityTestID(t, "12345678-1234-4abc-8def-1234567890ab")
	baseline, err := wireHushAdapterIdentityForTunnelID(id)
	if err != nil {
		t.Fatal(err)
	}
	for range []struct{ name, owner, text string }{{"Home", "S-1-5-18", "[Interface]"}, {"Office", "", "changed configuration"}} {
		identity, err := wireHushAdapterIdentityForTunnelID(id)
		if err != nil || identity != baseline {
			t.Fatalf("metadata changed identity: %#v, %v", identity, err)
		}
	}
}
