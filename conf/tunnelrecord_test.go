/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package conf

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

const tunnelRecordWGQuick = "# retained comment\n[Interface]\nPrivateKey = yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk=\nPostUp = echo retained # script comment\n"

func validPrivateTunnelRecord(t *testing.T) TunnelRecord {
	t.Helper()
	id, err := ParseTunnelID("12345678-1234-4abc-8def-1234567890ab")
	if err != nil {
		t.Fatal(err)
	}
	return TunnelRecord{TunnelID: id, Name: "Home", Scope: TunnelScopePrivate, OwnerSID: "S-1-5-18", WGQuickText: tunnelRecordWGQuick}
}

func validSharedTunnelRecord(t *testing.T) TunnelRecord {
	record := validPrivateTunnelRecord(t)
	record.Scope = TunnelScopeShared
	record.OwnerSID = ""
	return record
}

func TestTunnelRecordPrivateValid(t *testing.T) {
	if err := validPrivateTunnelRecord(t).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestTunnelRecordSharedValid(t *testing.T) {
	if err := validSharedTunnelRecord(t).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestTunnelRecordJSONRoundTrip(t *testing.T) {
	for _, record := range []TunnelRecord{validPrivateTunnelRecord(t), validSharedTunnelRecord(t)} {
		data, err := MarshalTunnelRecord(record)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := ParseTunnelRecord(data)
		if err != nil {
			t.Fatal(err)
		}
		if parsed != record {
			t.Fatalf("ParseTunnelRecord() = %#v, want %#v", parsed, record)
		}
		if parsed.WGQuickText != tunnelRecordWGQuick {
			t.Fatalf("WGQuickText = %q, want byte-for-byte original %q", parsed.WGQuickText, tunnelRecordWGQuick)
		}
	}
}

func TestTunnelRecordSerializedFields(t *testing.T) {
	for _, record := range []TunnelRecord{validPrivateTunnelRecord(t), validSharedTunnelRecord(t)} {
		data, err := MarshalTunnelRecord(record)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(data, []byte(`"storage_version":1`)) {
			t.Fatalf("storage version missing from %s", data)
		}
		if !bytes.Contains(data, []byte(`"tunnel_id":"12345678-1234-4abc-8def-1234567890ab"`)) {
			t.Fatalf("canonical TunnelID missing from %s", data)
		}
		if !bytes.Contains(data, []byte(`"scope":"`+record.Scope.String()+`"`)) {
			t.Fatalf("canonical scope missing from %s", data)
		}
	}
}

func TestTunnelRecordInvalidValidation(t *testing.T) {
	valid := validPrivateTunnelRecord(t)
	invalid := []TunnelRecord{
		{TunnelID: valid.TunnelID, Name: valid.Name, Scope: valid.Scope, OwnerSID: valid.OwnerSID, WGQuickText: ""},
		{TunnelID: valid.TunnelID, Name: "bad name", Scope: valid.Scope, OwnerSID: valid.OwnerSID, WGQuickText: valid.WGQuickText},
		{TunnelID: valid.TunnelID, Name: valid.Name, Scope: 0, OwnerSID: valid.OwnerSID, WGQuickText: valid.WGQuickText},
		{TunnelID: valid.TunnelID, Name: valid.Name, Scope: TunnelScope(3), OwnerSID: valid.OwnerSID, WGQuickText: valid.WGQuickText},
		{TunnelID: valid.TunnelID, Name: valid.Name, Scope: TunnelScopePrivate, OwnerSID: "", WGQuickText: valid.WGQuickText},
		{TunnelID: valid.TunnelID, Name: valid.Name, Scope: TunnelScopePrivate, OwnerSID: "not-a-sid", WGQuickText: valid.WGQuickText},
		{TunnelID: valid.TunnelID, Name: valid.Name, Scope: TunnelScopePrivate, OwnerSID: " S-1-5-18", WGQuickText: valid.WGQuickText},
		{TunnelID: valid.TunnelID, Name: valid.Name, Scope: TunnelScopePrivate, OwnerSID: "S-1-5-018", WGQuickText: valid.WGQuickText},
		{TunnelID: valid.TunnelID, Name: valid.Name, Scope: TunnelScopeShared, OwnerSID: "S-1-5-18", WGQuickText: valid.WGQuickText},
		{TunnelID: valid.TunnelID, Name: valid.Name, Scope: valid.Scope, OwnerSID: valid.OwnerSID, WGQuickText: "[Interface]\nPrivateKey = invalid\n"},
	}
	for _, record := range invalid {
		if err := record.Validate(); err == nil {
			t.Errorf("Validate(%#v) succeeded", record)
		}
		if _, err := MarshalTunnelRecord(record); err == nil {
			t.Errorf("MarshalTunnelRecord(%#v) succeeded", record)
		}
	}
	var zero TunnelID
	invalidID := valid
	invalidID.TunnelID = zero
	if err := invalidID.Validate(); err == nil {
		t.Fatal("zero TunnelID validated")
	}
}

func TestTunnelRecordParseRejectsInvalidJSON(t *testing.T) {
	data, err := MarshalTunnelRecord(validSharedTunnelRecord(t))
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"storage_version", "tunnel_id", "name", "scope", "owner_sid", "wg_quick_text"} {
		missing := make(map[string]json.RawMessage, len(fields)-1)
		for key, value := range fields {
			if key != field {
				missing[key] = value
			}
		}
		missingData, err := json.Marshal(missing)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ParseTunnelRecord(missingData); err == nil {
			t.Errorf("missing %q parsed successfully", field)
		}
	}
	for name, input := range map[string][]byte{
		"unknown version":        bytes.Replace(data, []byte(`"storage_version":1`), []byte(`"storage_version":2`), 1),
		"unknown field":          append(append([]byte{}, data[:len(data)-1]...), []byte(`,"unexpected_field":true}`)...),
		"malformed JSON":         []byte(`{"storage_version":`),
		"trailing second object": append(append([]byte{}, data...), []byte(` {}`)...),
		"trailing garbage":       append(append([]byte{}, data...), []byte(` garbage`)...),
		"uppercase TunnelID":     bytes.Replace(data, []byte(`12345678-1234-4abc-8def-1234567890ab`), []byte(`12345678-1234-4ABC-8def-1234567890ab`), 1),
		"uppercase scope":        bytes.Replace(data, []byte(`"scope":"shared"`), []byte(`"scope":"PRIVATE"`), 1),
	} {
		if _, err := ParseTunnelRecord(input); err == nil {
			t.Errorf("%s parsed successfully", name)
		}
	}
}

func TestTunnelScopeString(t *testing.T) {
	if TunnelScopePrivate.String() != "private" || TunnelScopeShared.String() != "shared" {
		t.Fatal("valid scope strings are incorrect")
	}
	if TunnelScope(0).String() != "" || TunnelScope(3).String() != "" {
		t.Fatal("invalid scope rendered as valid")
	}
	for _, text := range []string{"", "PRIVATE", "SHARED", "Private", "unknown"} {
		if _, err := parseTunnelScope(text); err == nil {
			t.Errorf("parseTunnelScope(%q) succeeded", text)
		}
	}
}

func TestTunnelRecordDoesNotMutateWGQuickText(t *testing.T) {
	record := validPrivateTunnelRecord(t)
	original := record.WGQuickText
	if err := record.Validate(); err != nil {
		t.Fatal(err)
	}
	if record.WGQuickText != original {
		t.Fatal("Validate mutated WGQuickText")
	}
	if _, err := MarshalTunnelRecord(record); err != nil {
		t.Fatal(err)
	}
	if record.WGQuickText != original {
		t.Fatal("MarshalTunnelRecord mutated WGQuickText")
	}
}

func TestTunnelRecordScopeJSONRejectsNonString(t *testing.T) {
	data, err := MarshalTunnelRecord(validSharedTunnelRecord(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range []string{"null", "1"} {
		input := bytes.Replace(data, []byte(`"scope":"shared"`), []byte(`"scope":`+scope), 1)
		if _, err := ParseTunnelRecord(input); err == nil {
			t.Errorf("scope %s parsed successfully", scope)
		}
	}
}

func TestTunnelRecordWGQuickTextIsNotNormalized(t *testing.T) {
	record := validPrivateTunnelRecord(t)
	data, err := MarshalTunnelRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseTunnelRecord(data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(parsed.WGQuickText, "# retained comment") || parsed.WGQuickText != record.WGQuickText {
		t.Fatalf("WGQuickText was normalized: %q", parsed.WGQuickText)
	}
}
