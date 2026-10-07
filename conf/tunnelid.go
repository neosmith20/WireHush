/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package conf

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
)

// TunnelID is the immutable UUIDv4 identity of a WireHush tunnel.
// Its zero value is invalid. String renders the zero value as the all-zero
// UUID for ordinary Go value semantics, so callers must use Valid when validity
// matters.
type TunnelID [16]byte

// NewTunnelID generates a cryptographically random UUIDv4 TunnelID.
func NewTunnelID() (TunnelID, error) {
	return newTunnelIDFromReader(rand.Reader)
}

func newTunnelIDFromReader(reader io.Reader) (TunnelID, error) {
	var id TunnelID
	if _, err := io.ReadFull(reader, id[:]); err != nil {
		return TunnelID{}, err
	}
	id[6] = id[6]&0x0f | 0x40
	id[8] = id[8]&0x3f | 0x80
	return id, nil
}

// ParseTunnelID accepts only canonical lowercase UUIDv4 text.
func ParseTunnelID(text string) (TunnelID, error) {
	if len(text) != 36 || text[8] != '-' || text[13] != '-' || text[18] != '-' || text[23] != '-' {
		return TunnelID{}, errors.New("TunnelID must be a canonical UUIDv4")
	}
	var token [32]byte
	for i, j := 0, 0; i < len(text); i++ {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue
		}
		if !isLowerHex(text[i]) {
			return TunnelID{}, errors.New("TunnelID must be a canonical UUIDv4")
		}
		token[j] = text[i]
		j++
	}
	var id TunnelID
	if _, err := hex.Decode(id[:], token[:]); err != nil || !id.Valid() {
		return TunnelID{}, errors.New("TunnelID must be a canonical UUIDv4")
	}
	return id, nil
}

func isLowerHex(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'f'
}

// Valid reports whether id is a non-zero UUIDv4 with the RFC UUID variant.
func (id TunnelID) Valid() bool {
	var zero TunnelID
	return id != zero && id[6]&0xf0 == 0x40 && id[8]&0xc0 == 0x80
}

// String returns id in canonical lowercase UUID text form. It does not validate id.
func (id TunnelID) String() string {
	var text [36]byte
	hex.Encode(text[0:8], id[0:4])
	text[8] = '-'
	hex.Encode(text[9:13], id[4:6])
	text[13] = '-'
	hex.Encode(text[14:18], id[6:8])
	text[18] = '-'
	hex.Encode(text[19:23], id[8:10])
	text[23] = '-'
	hex.Encode(text[24:36], id[10:16])
	return string(text[:])
}

// Token returns the lowercase hexadecimal TunnelID form used in system identifiers.
func (id TunnelID) Token() string {
	var token [32]byte
	hex.Encode(token[:], id[:])
	return string(token[:])
}
