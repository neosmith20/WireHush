/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package manager

import (
	"errors"
	"testing"

	"golang.zx2c4.com/wireguard/windows/conf"
)

const (
	wireHushAuthOwnerSID = "S-1-5-18"
	wireHushAuthOtherSID = "S-1-5-19"
)

func wireHushAuthTestID(t *testing.T) conf.TunnelID {
	t.Helper()
	id, err := conf.ParseTunnelID("12345678-1234-4abc-8def-1234567890ab")
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func wireHushAuthTestLocator(t *testing.T, scope conf.TunnelScope, ownerSID string) conf.TunnelServiceLocator {
	t.Helper()
	return conf.TunnelServiceLocator{Scope: scope, OwnerSID: ownerSID, TunnelID: wireHushAuthTestID(t)}
}

func TestWireHushCallerValidate(t *testing.T) {
	for _, test := range []struct {
		name   string
		caller wireHushCaller
		valid  bool
	}{
		{"canonical", wireHushCaller{SID: wireHushAuthOwnerSID}, true},
		{"empty", wireHushCaller{}, false},
		{"malformed", wireHushCaller{SID: "not-a-sid"}, false},
		{"noncanonical", wireHushCaller{SID: "S-1-5-018"}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.caller.Validate(); (err == nil) != test.valid {
				t.Fatalf("Validate() error = %v, valid = %v", err, test.valid)
			}
		})
	}
}

func TestAuthorizeWireHushPrivateTunnel(t *testing.T) {
	owner := wireHushCaller{SID: wireHushAuthOwnerSID}
	locator := wireHushAuthTestLocator(t, conf.TunnelScopePrivate, wireHushAuthOwnerSID)
	for _, operation := range []wireHushTunnelOperation{wireHushTunnelMetadataRead, wireHushTunnelStoredConfigRead, wireHushTunnelRuntimeConfigRead, wireHushTunnelControl, wireHushTunnelRecordMutation} {
		if err := authorizeWireHushTunnel(owner, locator, operation); err != nil {
			t.Fatalf("owner operation %d: %v", operation, err)
		}
	}
	for _, caller := range []wireHushCaller{
		{SID: wireHushAuthOtherSID},
		{SID: wireHushAuthOtherSID, Administrator: true},
		{SID: wireHushAuthOtherSID, WireHushUser: true},
	} {
		err := authorizeWireHushTunnel(caller, locator, wireHushTunnelControl)
		if !errors.Is(err, errWireHushAccessDenied) {
			t.Fatalf("caller %#v: error = %v", caller, err)
		}
	}
	if err := authorizeWireHushTunnel(owner, wireHushAuthTestLocator(t, conf.TunnelScopePrivate, wireHushAuthOtherSID), wireHushTunnelControl); !errors.Is(err, errWireHushAccessDenied) {
		t.Fatalf("same TunnelID with different owner: %v", err)
	}
}

func TestAuthorizeWireHushSharedTunnel(t *testing.T) {
	locator := wireHushAuthTestLocator(t, conf.TunnelScopeShared, "")
	admin := wireHushCaller{SID: wireHushAuthOwnerSID, Administrator: true}
	member := wireHushCaller{SID: wireHushAuthOtherSID, WireHushUser: true}
	for _, operation := range []wireHushTunnelOperation{wireHushTunnelMetadataRead, wireHushTunnelStoredConfigRead, wireHushTunnelRuntimeConfigRead, wireHushTunnelControl, wireHushTunnelRecordMutation} {
		if err := authorizeWireHushTunnel(admin, locator, operation); err != nil {
			t.Fatalf("administrator operation %d: %v", operation, err)
		}
	}
	for _, operation := range []wireHushTunnelOperation{wireHushTunnelMetadataRead, wireHushTunnelControl} {
		if err := authorizeWireHushTunnel(member, locator, operation); err != nil {
			t.Fatalf("member operation %d: %v", operation, err)
		}
	}
	for _, operation := range []wireHushTunnelOperation{wireHushTunnelStoredConfigRead, wireHushTunnelRuntimeConfigRead, wireHushTunnelRecordMutation} {
		if err := authorizeWireHushTunnel(member, locator, operation); !errors.Is(err, errWireHushAccessDenied) {
			t.Fatalf("member operation %d: %v", operation, err)
		}
	}
	for _, operation := range []wireHushTunnelOperation{wireHushTunnelMetadataRead, wireHushTunnelControl} {
		if err := authorizeWireHushTunnel(wireHushCaller{SID: wireHushAuthOtherSID}, locator, operation); !errors.Is(err, errWireHushAccessDenied) {
			t.Fatalf("unaffiliated operation %d: %v", operation, err)
		}
	}
}

func TestAuthorizeWireHushNamespace(t *testing.T) {
	owner := wireHushCaller{SID: wireHushAuthOwnerSID}
	admin := wireHushCaller{SID: wireHushAuthOtherSID, Administrator: true}
	member := wireHushCaller{SID: wireHushAuthOtherSID, WireHushUser: true}
	if err := authorizeWireHushNamespace(owner, conf.TunnelScopePrivate, wireHushAuthOwnerSID, wireHushTunnelRecordMutation); err != nil {
		t.Fatal(err)
	}
	for _, caller := range []wireHushCaller{{SID: wireHushAuthOtherSID}, admin} {
		if err := authorizeWireHushNamespace(caller, conf.TunnelScopePrivate, wireHushAuthOwnerSID, wireHushTunnelMetadataRead); !errors.Is(err, errWireHushAccessDenied) {
			t.Fatalf("private caller %#v: %v", caller, err)
		}
	}
	if err := authorizeWireHushNamespace(admin, conf.TunnelScopeShared, "", wireHushTunnelRecordMutation); err != nil {
		t.Fatal(err)
	}
	if err := authorizeWireHushNamespace(member, conf.TunnelScopeShared, "", wireHushTunnelControl); err != nil {
		t.Fatal(err)
	}
	if err := authorizeWireHushNamespace(member, conf.TunnelScopeShared, "", wireHushTunnelRecordMutation); !errors.Is(err, errWireHushAccessDenied) {
		t.Fatal(err)
	}
	for _, namespace := range []struct {
		scope conf.TunnelScope
		owner string
	}{
		{conf.TunnelScopeShared, wireHushAuthOwnerSID},
		{conf.TunnelScopePrivate, ""},
		{conf.TunnelScopePrivate, "not-a-sid"},
	} {
		err := authorizeWireHushNamespace(owner, namespace.scope, namespace.owner, wireHushTunnelMetadataRead)
		if err == nil || errors.Is(err, errWireHushAccessDenied) {
			t.Fatalf("namespace %#v error = %v", namespace, err)
		}
	}
}

func TestAuthorizeWireHushTunnelRejectsInvalidOperation(t *testing.T) {
	caller := wireHushCaller{SID: wireHushAuthOwnerSID}
	locator := wireHushAuthTestLocator(t, conf.TunnelScopePrivate, wireHushAuthOwnerSID)
	for _, operation := range []wireHushTunnelOperation{0, wireHushTunnelOperation(99)} {
		err := authorizeWireHushTunnel(caller, locator, operation)
		if err == nil || errors.Is(err, errWireHushAccessDenied) {
			t.Fatalf("operation %d error = %v", operation, err)
		}
	}
}
