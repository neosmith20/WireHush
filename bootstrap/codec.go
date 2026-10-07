/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */
package bootstrap

import (
	"encoding/json"
	"errors"
)

func MarshalSettings(settings Settings) ([]byte, error) {
	if len(settings.Entries) > 64 {
		return nil, errors.New("too many bootstrap resolvers")
	}
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	encoded := make([]entryJSON, len(settings.Entries))
	for i, entry := range settings.Entries {
		encoded[i] = entryJSON{Address: entry.Address.String(), Enabled: entry.Enabled, Custom: entry.Custom}
	}
	return json.MarshalIndent(encoded, "", "  ")
}
func ParseSettings(data []byte) (Settings, error) {
	if len(data) > 64*1024 {
		return Settings{}, errors.New("bootstrap settings exceed size limit")
	}
	var encoded []entryJSON
	if err := json.Unmarshal(data, &encoded); err != nil {
		return Settings{}, errors.New("invalid bootstrap settings encoding")
	}
	if len(encoded) > 64 {
		return Settings{}, errors.New("too many bootstrap resolvers")
	}
	settings := Settings{Entries: make([]Entry, len(encoded))}
	for i, entry := range encoded {
		address, err := ParseAddress(entry.Address)
		if err != nil {
			return Settings{}, err
		}
		settings.Entries[i] = Entry{Address: address, Enabled: entry.Enabled, Custom: entry.Custom}
	}
	if err := settings.Validate(); err != nil {
		return Settings{}, err
	}
	return settings, nil
}
