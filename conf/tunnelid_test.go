/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package conf

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestTunnelIDGeneratedUUIDv4(t *testing.T) {
	id, err := NewTunnelID()
	if err != nil {
		t.Fatal(err)
	}
	if !id.Valid() {
		t.Fatalf("generated invalid TunnelID: %s", id)
	}
	if id[6]&0xf0 != 0x40 {
		t.Fatalf("version bits = %x, want 4", id[6]>>4)
	}
	if id[8]&0xc0 != 0x80 {
		t.Fatalf("variant bits = %b, want 10", id[8]>>6)
	}
}

func TestTunnelIDCanonicalStringAndToken(t *testing.T) {
	id, err := ParseTunnelID("12345678-1234-4abc-8def-1234567890ab")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := id.String(), "12345678-1234-4abc-8def-1234567890ab"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
	if got, want := id.Token(), "1234567812344abc8def1234567890ab"; got != want {
		t.Fatalf("Token() = %q, want %q", got, want)
	}
	if len(id.Token()) != 32 || strings.Contains(id.Token(), "-") {
		t.Fatalf("Token() = %q, want 32 lowercase hex characters", id.Token())
	}
}

func TestTunnelIDRoundTrip(t *testing.T) {
	id, err := NewTunnelID()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseTunnelID(id.String())
	if err != nil {
		t.Fatal(err)
	}
	if parsed != id {
		t.Fatalf("ParseTunnelID(String()) = %s, want %s", parsed, id)
	}
}

func TestTunnelIDReaderVector(t *testing.T) {
	id, err := newTunnelIDFromReader(bytes.NewReader([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := id.String(), "00010203-0405-4607-8809-0a0b0c0d0e0f"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestTunnelIDReaderErrors(t *testing.T) {
	boom := errors.New("entropy unavailable")
	if _, err := newTunnelIDFromReader(errorReader{boom}); !errors.Is(err, boom) {
		t.Fatalf("error = %v, want %v", err, boom)
	}
	if _, err := newTunnelIDFromReader(bytes.NewReader(make([]byte, 15))); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("short read error = %v, want %v", err, io.ErrUnexpectedEOF)
	}
}

func TestTunnelIDParseRejectsNonCanonicalValues(t *testing.T) {
	valid := "12345678-1234-4abc-8def-1234567890ab"
	for _, text := range []string{
		"",
		strings.ToUpper(valid),
		"1234567812344abc8def1234567890ab",
		"{12345678-1234-4abc-8def-1234567890ab}",
		" " + valid,
		valid + " ",
		"12345678-1234-4abc-8def-1234567890ag",
		"12345678-1234-5abc-8def-1234567890ab",
		"12345678-1234-4abc-7def-1234567890ab",
		"00000000-0000-0000-0000-000000000000",
	} {
		if _, err := ParseTunnelID(text); err == nil {
			t.Errorf("ParseTunnelID(%q) succeeded", text)
		}
	}
}

func TestTunnelIDZeroValue(t *testing.T) {
	var id TunnelID
	if id.Valid() {
		t.Fatal("zero TunnelID is valid")
	}
	if got, want := id.String(), "00000000-0000-0000-0000-000000000000"; got != want {
		t.Fatalf("zero String() = %q, want %q", got, want)
	}
}

type errorReader struct{ err error }

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }
