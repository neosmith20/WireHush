/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package manager

import (
	"errors"
	"os"
	"testing"

	"golang.zx2c4.com/wireguard/windows/conf"
)

const wireHushControlWGQuick = "[Interface]\nPrivateKey = AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\nAddress = 10.0.0.2/32\n"

func wireHushControlRecord(t *testing.T, id string, scope conf.TunnelScope, ownerSID, name string) conf.TunnelRecord {
	t.Helper()
	tunnelID, err := conf.ParseTunnelID(id)
	if err != nil {
		t.Fatal(err)
	}
	return conf.TunnelRecord{TunnelID: tunnelID, Scope: scope, OwnerSID: ownerSID, Name: name, WGQuickText: wireHushControlWGQuick}
}

func wireHushControlLocator(record conf.TunnelRecord) conf.TunnelServiceLocator {
	return conf.TunnelServiceLocator{Scope: record.Scope, OwnerSID: record.OwnerSID, TunnelID: record.TunnelID}
}

func wireHushControlForRecords(records ...conf.TunnelRecord) (wireHushManagerControl, *map[conf.TunnelServiceLocator]conf.TunnelRecord, *int) {
	stored := make(map[conf.TunnelServiceLocator]conf.TunnelRecord, len(records))
	for _, record := range records {
		stored[wireHushControlLocator(record)] = record
	}
	loads := 0
	load := func(scope conf.TunnelScope, owner string, id conf.TunnelID) (conf.TunnelRecord, error) {
		loads++
		record, ok := stored[conf.TunnelServiceLocator{Scope: scope, OwnerSID: owner, TunnelID: id}]
		if !ok {
			return conf.TunnelRecord{}, os.ErrNotExist
		}
		return record, nil
	}
	control := wireHushManagerControl{
		loadRecord: load,
		listRecords: func(scope conf.TunnelScope, owner string) ([]conf.TunnelRecord, error) {
			result := []conf.TunnelRecord{}
			for _, record := range stored {
				if record.Scope == scope && record.OwnerSID == owner {
					result = append(result, record)
				}
			}
			return result, nil
		},
		saveRecord: func(record conf.TunnelRecord, overwrite bool) error {
			stored[wireHushControlLocator(record)] = record
			return nil
		},
		deleteRecord: func(scope conf.TunnelScope, owner string, id conf.TunnelID) error {
			delete(stored, conf.TunnelServiceLocator{Scope: scope, OwnerSID: owner, TunnelID: id})
			return nil
		},
		storedConfig:  func(conf.TunnelServiceLocator) (*conf.Config, error) { return &conf.Config{Name: "secret"}, nil },
		runtimeConfig: func(conf.TunnelServiceLocator) (*conf.Config, error) { return &conf.Config{Name: "runtime"}, nil },
		start:         func(conf.TunnelServiceLocator) error { return nil },
		stop:          func(conf.TunnelServiceLocator) error { return nil },
		waitForStop:   func(conf.TunnelServiceLocator) error { return nil },
		state:         func(conf.TunnelServiceLocator) (TunnelState, error) { return TunnelStarted, nil },
	}
	return control, &stored, &loads
}

func TestWireHushManagerControlPrivateAuthorization(t *testing.T) {
	record := wireHushControlRecord(t, "12345678-1234-4abc-8def-1234567890ab", conf.TunnelScopePrivate, wireHushAuthOwnerSID, "Home")
	control, _, loads := wireHushControlForRecords(record)
	owner := wireHushCaller{SID: wireHushAuthOwnerSID}
	locator := wireHushControlLocator(record)
	if metadata, err := control.TunnelMetadata(owner, locator); err != nil || metadata.Name != "Home" {
		t.Fatalf("metadata = %#v, %v", metadata, err)
	}
	if config, err := control.StoredTunnelConfig(owner, locator); err != nil || config.Name != "secret" {
		t.Fatalf("stored config = %#v, %v", config, err)
	}
	if config, err := control.RuntimeTunnelConfig(owner, locator); err != nil || config.Name != "runtime" {
		t.Fatalf("runtime config = %#v, %v", config, err)
	}
	if state, err := control.TunnelState(owner, locator); err != nil || state != TunnelStarted {
		t.Fatalf("state = %v, %v", state, err)
	}
	if err := control.StartTunnel(owner, locator); err != nil {
		t.Fatal(err)
	}
	if err := control.StopTunnel(owner, locator); err != nil {
		t.Fatal(err)
	}
	if err := control.WaitForTunnelStop(owner, locator); err != nil {
		t.Fatal(err)
	}
	if err := control.SaveTunnelRecord(owner, locator, record); err != nil {
		t.Fatal(err)
	}
	for _, caller := range []wireHushCaller{{SID: wireHushAuthOtherSID}, {SID: wireHushAuthOtherSID, Administrator: true}, {SID: wireHushAuthOtherSID, WireHushUser: true}} {
		before := *loads
		if _, err := control.StoredTunnelConfig(caller, locator); !errors.Is(err, errWireHushAccessDenied) || *loads != before {
			t.Fatalf("caller %#v error = %v, loads = %d", caller, err, *loads)
		}
	}
	otherName := record
	otherName.Name = "Office"
	if _, err := control.TunnelMetadata(owner, wireHushControlLocator(otherName)); err != nil {
		t.Fatal(err)
	}
	differentOwner := record
	differentOwner.OwnerSID = wireHushAuthOtherSID
	if _, err := control.TunnelMetadata(owner, wireHushControlLocator(differentOwner)); !errors.Is(err, errWireHushAccessDenied) {
		t.Fatalf("TunnelID-only access error = %v", err)
	}
}

func TestWireHushManagerControlSharedAuthorization(t *testing.T) {
	record := wireHushControlRecord(t, "12345678-1234-4abc-8def-1234567890ab", conf.TunnelScopeShared, "", "Shared")
	control, _, _ := wireHushControlForRecords(record)
	locator := wireHushControlLocator(record)
	admin := wireHushCaller{SID: wireHushAuthOwnerSID, Administrator: true}
	member := wireHushCaller{SID: wireHushAuthOtherSID, WireHushUser: true}
	for _, call := range []func() error{
		func() error { _, err := control.TunnelMetadata(admin, locator); return err },
		func() error { _, err := control.StoredTunnelConfig(admin, locator); return err },
		func() error { _, err := control.RuntimeTunnelConfig(admin, locator); return err },
		func() error { return control.StartTunnel(admin, locator) },
		func() error { return control.StopTunnel(admin, locator) },
		func() error { return control.SaveTunnelRecord(admin, locator, record) },
	} {
		if err := call(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := control.TunnelMetadata(member, locator); err != nil {
		t.Fatal(err)
	}
	if _, err := control.TunnelState(member, locator); err != nil {
		t.Fatal(err)
	}
	if err := control.StartTunnel(member, locator); err != nil {
		t.Fatal(err)
	}
	if err := control.StopTunnel(member, locator); err != nil {
		t.Fatal(err)
	}
	if err := control.WaitForTunnelStop(member, locator); err != nil {
		t.Fatal(err)
	}
	for _, call := range []func() error{
		func() error { _, err := control.StoredTunnelConfig(member, locator); return err },
		func() error { _, err := control.RuntimeTunnelConfig(member, locator); return err },
		func() error { return control.SaveTunnelRecord(member, locator, record) },
		func() error { return control.DeleteTunnelRecord(member, locator) },
	} {
		if err := call(); !errors.Is(err, errWireHushAccessDenied) {
			t.Fatalf("member error = %v", err)
		}
	}
	if err := control.StartTunnel(wireHushCaller{SID: wireHushAuthOtherSID}, locator); !errors.Is(err, errWireHushAccessDenied) {
		t.Fatalf("unaffiliated error = %v", err)
	}
}

func TestWireHushManagerControlNamespaceAndErrors(t *testing.T) {
	private := wireHushControlRecord(t, "12345678-1234-4abc-8def-1234567890ab", conf.TunnelScopePrivate, wireHushAuthOwnerSID, "Home")
	control, _, _ := wireHushControlForRecords(private)
	owner := wireHushCaller{SID: wireHushAuthOwnerSID}
	if records, err := control.ListTunnelMetadata(owner, conf.TunnelScopePrivate, wireHushAuthOwnerSID); err != nil || len(records) != 1 || records[0].Name != "Home" {
		t.Fatalf("records = %#v, %v", records, err)
	}
	for _, caller := range []wireHushCaller{{SID: wireHushAuthOtherSID}, {SID: wireHushAuthOtherSID, Administrator: true}, {SID: wireHushAuthOtherSID, WireHushUser: true}} {
		records, err := control.ListTunnelMetadata(caller, conf.TunnelScopePrivate, wireHushAuthOwnerSID)
		if !errors.Is(err, errWireHushAccessDenied) || records != nil {
			t.Fatalf("private list caller %#v records = %#v, error = %v", caller, records, err)
		}
	}
	if _, err := control.ListTunnelMetadata(owner, conf.TunnelScopeShared, wireHushAuthOwnerSID); err == nil || errors.Is(err, errWireHushAccessDenied) {
		t.Fatalf("invalid namespace error = %v", err)
	}
	if _, err := control.TunnelMetadata(wireHushCaller{SID: "S-1-5-018"}, wireHushControlLocator(private)); err == nil || errors.Is(err, errWireHushAccessDenied) {
		t.Fatalf("invalid caller error = %v", err)
	}
	unknown := private
	unknown.TunnelID, _ = conf.ParseTunnelID("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	if _, err := control.TunnelMetadata(owner, wireHushControlLocator(unknown)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unknown TunnelID error = %v", err)
	}
	invalid := wireHushControlLocator(private)
	invalid.OwnerSID = ""
	if _, err := control.TunnelMetadata(owner, invalid); err == nil || errors.Is(err, errWireHushAccessDenied) {
		t.Fatalf("invalid locator error = %v", err)
	}
	mismatched := private
	mismatched.OwnerSID = wireHushAuthOtherSID
	mismatchControl, _, _ := wireHushControlForRecords(private)
	mismatchControl.loadRecord = func(conf.TunnelScope, string, conf.TunnelID) (conf.TunnelRecord, error) { return mismatched, nil }
	if _, _, err := mismatchControl.resolveAuthorizedTunnel(owner, wireHushControlLocator(private), wireHushTunnelMetadataRead); err == nil || errors.Is(err, errWireHushAccessDenied) {
		t.Fatalf("stored identity mismatch error = %v", err)
	}
}

func TestWireHushManagerControlUpdateRejectsIdentityChange(t *testing.T) {
	record := wireHushControlRecord(t, "12345678-1234-4abc-8def-1234567890ab", conf.TunnelScopePrivate, wireHushAuthOwnerSID, "Home")
	control, _, _ := wireHushControlForRecords(record)
	saves := 0
	control.saveRecord = func(conf.TunnelRecord, bool) error { saves++; return nil }
	owner := wireHushCaller{SID: wireHushAuthOwnerSID}
	locator := wireHushControlLocator(record)
	otherID := wireHushControlRecord(t, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", conf.TunnelScopePrivate, wireHushAuthOwnerSID, "Home")
	shared := record
	shared.Scope = conf.TunnelScopeShared
	shared.OwnerSID = ""
	otherOwner := record
	otherOwner.OwnerSID = wireHushAuthOtherSID
	for _, replacement := range []conf.TunnelRecord{otherID, shared, otherOwner} {
		if err := control.SaveTunnelRecord(owner, locator, replacement); err == nil {
			t.Fatalf("identity-changing replacement %#v succeeded", replacement)
		}
		if saves != 0 {
			t.Fatalf("identity-changing replacement mutated storage %d times", saves)
		}
	}
	updated := record
	updated.Name = "Office"
	if err := control.SaveTunnelRecord(owner, locator, updated); err != nil {
		t.Fatal(err)
	}
	if saves != 1 {
		t.Fatalf("identity-preserving update saves = %d, want 1", saves)
	}
}
