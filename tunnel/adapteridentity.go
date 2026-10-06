/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package tunnel

import (
	"encoding/binary"
	"errors"

	"golang.org/x/crypto/blake2s"
	"golang.org/x/sys/windows"

	"golang.zx2c4.com/wireguard/windows/conf"
	"golang.zx2c4.com/wireguard/windows/driver"
)

type wireHushAdapterIdentity struct {
	Name string
	GUID windows.GUID
}

func wireHushAdapterIdentityForTunnelID(id conf.TunnelID) (wireHushAdapterIdentity, error) {
	name, err := conf.WireHushAdapterNameOfTunnelID(id)
	if err != nil {
		return wireHushAdapterIdentity{}, err
	}
	if len(name) >= driver.AdapterNameMax {
		return wireHushAdapterIdentity{}, errors.New("WireHush adapter name is too long")
	}
	idString := id.String()
	var length [4]byte
	binary.LittleEndian.PutUint32(length[:], uint32(len(idString)))
	digest := blake2s.Sum256(append(append([]byte(deterministicGUIDLabel), length[:]...), idString...))
	return wireHushAdapterIdentity{
		Name: name,
		GUID: windows.GUID{
			Data1: binary.LittleEndian.Uint32(digest[0:4]),
			Data2: binary.LittleEndian.Uint16(digest[4:6]),
			Data3: binary.LittleEndian.Uint16(digest[6:8]),
			Data4: [8]byte(digest[8:16]),
		},
	}, nil
}
