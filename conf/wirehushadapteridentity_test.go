/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package conf

import "testing"

func TestWireHushAdapterNameOfTunnelID(t *testing.T) {
	id, err := ParseTunnelID("12345678-1234-4abc-8def-1234567890ab")
	if err != nil {
		t.Fatal(err)
	}
	name, err := WireHushAdapterNameOfTunnelID(id)
	if err != nil {
		t.Fatal(err)
	}
	const want = "WireHush-1234567812344abc8def1234567890ab"
	if name != want {
		t.Fatalf("name = %q, want %q", name, want)
	}
	if len(name) != 41 {
		t.Fatalf("name length = %d, want 41", len(name))
	}
	if name[9:] != id.Token() {
		t.Fatalf("name token = %q, want %q", name[9:], id.Token())
	}
}

func TestWireHushAdapterNameOfTunnelIDRejectsInvalidID(t *testing.T) {
	if _, err := WireHushAdapterNameOfTunnelID(TunnelID{}); err == nil {
		t.Fatal("zero TunnelID accepted")
	}
}
