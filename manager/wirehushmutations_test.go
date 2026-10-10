/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package manager

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.zx2c4.com/wireguard/windows/conf"
)

func testWireHushMutations(records ...conf.TunnelRecord) *wireHushMutations {
	control, _, _ := wireHushControlForRecords(records...)
	return &wireHushMutations{control: control, gate: make(chan struct{}, 1), inventory: func() ([]wireHushTrackedTunnel, error) { return nil, nil }}
}

func TestWireHushConcurrentStartsAdmitOnlyOne(t *testing.T) {
	a := wireHushControlRecord(t, "12345678-1234-4abc-8def-1234567890ab", conf.TunnelScopePrivate, wireHushAuthOwnerSID, "A")
	b := wireHushControlRecord(t, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", conf.TunnelScopePrivate, wireHushAuthOtherSID, "B")
	mutations := testWireHushMutations(a, b)
	var active []wireHushTrackedTunnel
	starts := 0
	mutations.inventory = func() ([]wireHushTrackedTunnel, error) { return active, nil }
	mutations.control.start = func(locator conf.TunnelServiceLocator) error {
		starts++
		active = []wireHushTrackedTunnel{{Locator: locator, State: TunnelStarting}}
		return nil
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, record := range []conf.TunnelRecord{a, b} {
		wg.Add(1)
		go func(record conf.TunnelRecord) {
			defer wg.Done()
			results <- mutations.Start(context.Background(), wireHushCaller{SID: record.OwnerSID}, wireHushControlLocator(record))
		}(record)
	}
	wg.Wait()
	close(results)
	success, busy := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, errWireHushDeviceBusy) {
			busy++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || busy != 1 || starts != 1 {
		t.Fatalf("success=%d busy=%d starts=%d", success, busy, starts)
	}
}

func TestWireHushAdmissionFailsClosedAndNeverStopsOtherTunnel(t *testing.T) {
	record := wireHushControlRecord(t, "12345678-1234-4abc-8def-1234567890ab", conf.TunnelScopePrivate, wireHushAuthOwnerSID, "A")
	other := wireHushControlRecord(t, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", conf.TunnelScopePrivate, wireHushAuthOtherSID, "SECRET-NAME")
	owner := wireHushCaller{SID: record.OwnerSID}
	mutations := testWireHushMutations(record, other)
	mutations.control.start = func(conf.TunnelServiceLocator) error { t.Fatal("start bypassed busy gate"); return nil }
	mutations.control.stop = func(conf.TunnelServiceLocator) error { t.Fatal("start stopped another user's tunnel"); return nil }
	for _, state := range []TunnelState{TunnelStarting, TunnelStarted, TunnelStopping, TunnelUnknown} {
		mutations.inventory = func() ([]wireHushTrackedTunnel, error) {
			return []wireHushTrackedTunnel{{Locator: wireHushControlLocator(other), State: state}}, nil
		}
		if err := mutations.Start(context.Background(), owner, wireHushControlLocator(record)); err != errWireHushDeviceBusy {
			t.Fatalf("other active state=%v error=%v", state, err)
		}
	}
	if err := mutations.Start(context.Background(), owner, wireHushControlLocator(other)); !errors.Is(err, errWireHushAccessDenied) {
		t.Fatal(err)
	}
	inventoryError := errors.New("inventory unavailable")
	mutations.inventory = func() ([]wireHushTrackedTunnel, error) { return nil, inventoryError }
	if err := mutations.Start(context.Background(), owner, wireHushControlLocator(record)); !errors.Is(err, inventoryError) {
		t.Fatal(err)
	}
}

func TestWireHushMutationGateCancellationAndActiveUpdate(t *testing.T) {
	record := wireHushControlRecord(t, "12345678-1234-4abc-8def-1234567890ab", conf.TunnelScopePrivate, wireHushAuthOwnerSID, "A")
	mutations := testWireHushMutations(record)
	owner := wireHushCaller{SID: record.OwnerSID}
	locator := wireHushControlLocator(record)
	mutations.gate <- struct{}{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	err := mutations.Stop(ctx, owner, locator)
	cancel()
	mutations.leave()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("queued mutation error=%v", err)
	}
	mutations.control.state = func(conf.TunnelServiceLocator) (TunnelState, error) { return TunnelStarting, nil }
	mutations.control.saveRecord = func(conf.TunnelRecord, bool) error { t.Fatal("transitional update wrote record"); return nil }
	if err := mutations.Update(context.Background(), owner, locator, "B", record.WGQuickText); !errors.Is(err, errWireHushDeviceBusy) {
		t.Fatal(err)
	}
	mutations.closing = true
	if err := mutations.Stop(context.Background(), owner, locator); !errors.Is(err, errWireHushClosing) {
		t.Fatal(err)
	}
}

func TestWireHushCreateDerivesIdentityAndRestrictsShared(t *testing.T) {
	mutations := testWireHushMutations()
	owner := wireHushCaller{SID: wireHushAuthOwnerSID, WireHushUser: true}
	a, err := mutations.Create(context.Background(), owner, conf.TunnelScopePrivate, "A", wireHushControlWGQuick)
	if err != nil || !a.TunnelID.Valid() || a.OwnerSID != owner.SID {
		t.Fatalf("private create=%+v, %v", a, err)
	}
	b, err := mutations.Create(context.Background(), owner, conf.TunnelScopePrivate, "B", wireHushControlWGQuick)
	if err != nil || a.TunnelID == b.TunnelID {
		t.Fatalf("new import identity=%+v, %v", b, err)
	}
	if _, err := mutations.Create(context.Background(), owner, conf.TunnelScopeShared, "Shared", wireHushControlWGQuick); !errors.Is(err, errWireHushAccessDenied) {
		t.Fatal(err)
	}
}

func TestWireHushUpdateLifecycle(t *testing.T) {
	for _, active := range []bool{false, true} {
		for _, failure := range []string{"", "stop", "save", "start", "invalid"} {
			t.Run(fmt.Sprintf("active=%v/%s", active, failure), func(t *testing.T) {
				record := wireHushControlRecord(t, "12345678-1234-4abc-8def-1234567890ab", conf.TunnelScopePrivate, wireHushAuthOwnerSID, "A")
				m := testWireHushMutations(record)
				var calls []string
				m.control.state = func(conf.TunnelServiceLocator) (TunnelState, error) {
					if active {
						return TunnelStarted, nil
					}
					return TunnelStopped, nil
				}
				m.control.stop = func(conf.TunnelServiceLocator) error {
					calls = append(calls, "stop")
					if failure == "stop" {
						return errors.New("stop")
					}
					return nil
				}
				m.control.waitForStop = func(conf.TunnelServiceLocator) error { calls = append(calls, "wait"); return nil }
				save := m.control.saveRecord
				m.control.saveRecord = func(r conf.TunnelRecord, overwrite bool) error {
					calls = append(calls, "save")
					if failure == "save" {
						return errors.New("save")
					}
					return save(r, overwrite)
				}
				m.control.start = func(conf.TunnelServiceLocator) error {
					calls = append(calls, "start")
					if failure == "start" {
						return errors.New("start")
					}
					return nil
				}
				text := record.WGQuickText
				if failure == "invalid" {
					text = "invalid"
				}
				err := m.Update(context.Background(), wireHushCaller{SID: record.OwnerSID}, wireHushControlLocator(record), "B", text)
				expected := "save"
				if active {
					expected = "stop,wait,save,start"
				}
				if failure == "invalid" {
					expected = ""
				} else if active && failure == "stop" {
					expected = "stop"
				} else if failure == "save" {
					expected = "save"
					if active {
						expected = "stop,wait,save"
					}
				}
				if strings.Join(calls, ",") != expected {
					t.Fatalf("calls=%v expected=%s", calls, expected)
				}
				saved, loadErr := m.control.loadRecord(record.Scope, record.OwnerSID, record.TunnelID)
				if loadErr != nil {
					t.Fatal(loadErr)
				}
				persisted := failure != "invalid" && failure != "save" && !(active && failure == "stop")
				if (saved.Name == "B") != persisted {
					t.Fatalf("saved name=%s", saved.Name)
				}
				if active && failure == "start" && !errors.Is(err, errWireHushReconnectFailed) {
					t.Fatalf("reconnect error=%v", err)
				}
				if failure == "" && err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
