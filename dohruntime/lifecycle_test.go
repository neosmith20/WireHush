/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 TunnelMint contributors. All Rights Reserved.
 */

package dohruntime

import (
	"context"
	"errors"
	"io"
	"net/netip"
	"reflect"
	"testing"
)

type closeFunc func() error

func (f closeFunc) Close() error { return f() }

func TestCloseRetriesFailedDNSRestorationInsteadOfFalseSuccess(t *testing.T) {
	fail, restored, proxyClosed := true, 0, 0
	s := &Session{restore: func() error {
		restored++
		if fail {
			return errors.New("DNS restore failed")
		}
		return nil
	}, proxy: closeFunc(func() error { proxyClosed++; return nil })}
	if err := s.Close(); err == nil {
		t.Fatal("failed restoration was acknowledged")
	}
	if err := s.Close(); err == nil {
		t.Fatal("retry forgot failed DNS restoration")
	}
	fail = false
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil || restored != 3 || proxyClosed != 1 {
		t.Fatal("cleanup was not idempotent after successful restoration")
	}
}

func TestActivateOrdersStagesAndOwnsRoutes(t *testing.T) {
	var events []string
	var restored, closed bool
	endpoint := "https://dns.example/dns-query"
	candidates := []netip.Addr{netip.MustParseAddr("203.0.113.9"), netip.MustParseAddr("2001:db8::9")}
	hooks := Hooks{
		Bootstrap: func(_ context.Context, got string) ([]netip.Addr, error) {
			events = append(events, "bootstrap:"+got)
			return candidates, nil
		},
		AddRoute: func(prefix netip.Prefix) (bool, error) {
			events = append(events, "add:"+prefix.String())
			return true, nil
		},
		DelRoute: func(prefix netip.Prefix) error {
			events = append(events, "del:"+prefix.String())
			return nil
		},
		Verify: func(_ context.Context, got string, addresses []netip.Addr) error {
			events = append(events, "verify:"+got+":"+string(rune(len(addresses))))
			return nil
		},
		StartProxy: func(_ context.Context, got string, _ []netip.Addr) (io.Closer, error) {
			events = append(events, "proxy:"+got)
			return closeFunc(func() error { closed = true; events = append(events, "close-proxy"); return nil }), nil
		},
		SetDNS: func() (func() error, error) {
			events = append(events, "dns")
			return func() error { restored = true; events = append(events, "restore-dns"); return nil }, nil
		},
	}
	s, err := Activate(context.Background(), Config{Endpoint: endpoint}, hooks)
	if err != nil {
		t.Fatal(err)
	}
	if s.Stage() != StageReady {
		t.Fatalf("stage = %v, want ready", s.Stage())
	}
	if got, want := s.Routes(), []netip.Prefix{netip.MustParsePrefix("203.0.113.9/32"), netip.MustParsePrefix("2001:db8::9/128")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("routes = %v, want %v", got, want)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if !restored || !closed {
		t.Fatalf("cleanup flags restored=%v closed=%v", restored, closed)
	}
	if got, want := events[len(events)-4:], []string{"restore-dns", "close-proxy", "del:2001:db8::9/128", "del:203.0.113.9/32"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("cleanup order = %v, want %v", got, want)
	}
}

func TestActivateRejectsRouteRecursionAndRollsBack(t *testing.T) {
	var added, deleted int
	hooks := Hooks{
		Bootstrap: func(context.Context, string) ([]netip.Addr, error) {
			return []netip.Addr{netip.MustParseAddr("192.0.2.7")}, nil
		},
		AddRoute: func(netip.Prefix) (bool, error) { added++; return true, nil },
		DelRoute: func(netip.Prefix) error { deleted++; return nil },
		Verify:   func(context.Context, string, []netip.Addr) error { return errors.New("transport failed") },
		StartProxy: func(context.Context, string, []netip.Addr) (io.Closer, error) {
			t.Fatal("proxy started after failed verify")
			return nil, nil
		},
		SetDNS: func() (func() error, error) { t.Fatal("DNS configured after failed verify"); return nil, nil },
	}
	if _, err := Activate(context.Background(), Config{Endpoint: "https://dns.example/dns-query", PeerEndpointAddress: []netip.Addr{netip.MustParseAddr("192.0.2.7")}}, hooks); err == nil {
		t.Fatal("route recursion was accepted")
	}
	if added != 0 || deleted != 0 {
		t.Fatalf("route recursion changed routes: added=%d deleted=%d", added, deleted)
	}

	hooks.Bootstrap = func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("192.0.2.8")}, nil
	}
	if _, err := Activate(context.Background(), Config{Endpoint: "https://dns.example/dns-query"}, hooks); err == nil {
		t.Fatal("failed verification unexpectedly succeeded")
	}
	if added != 1 || deleted != 1 {
		t.Fatalf("failed activation did not roll back route: added=%d deleted=%d", added, deleted)
	}
}

func TestSelectRouteCandidatesFiltersUnsafeAndDeduplicates(t *testing.T) {
	got, err := selectRouteCandidates([]netip.Addr{
		netip.MustParseAddr("224.0.0.1"),
		netip.MustParseAddr("0.0.0.0"),
		netip.MustParseAddr("198.51.100.2"),
		netip.MustParseAddr("198.51.100.2"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []netip.Addr{netip.MustParseAddr("198.51.100.2")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("candidates = %v, want %v", got, want)
	}
}

func TestRefreshReplacesOwnedRoutesAndProxy(t *testing.T) {
	var added, deleted []string
	var swaps int
	hooks := Hooks{
		Bootstrap: func(context.Context, string) ([]netip.Addr, error) {
			return []netip.Addr{netip.MustParseAddr("192.0.2.1")}, nil
		},
		AddRoute: func(prefix netip.Prefix) (bool, error) {
			added = append(added, prefix.String())
			return true, nil
		},
		DelRoute: func(prefix netip.Prefix) error {
			deleted = append(deleted, prefix.String())
			return nil
		},
		Verify: func(context.Context, string, []netip.Addr) error { return nil },
		StartProxy: func(context.Context, string, []netip.Addr) (io.Closer, error) {
			return closeFunc(func() error { return nil }), nil
		},
		ReplaceProxy: func(context.Context, string, []netip.Addr, io.Closer) error {
			swaps++
			return nil
		},
		SetDNS: func() (func() error, error) { return func() error { return nil }, nil },
	}
	s, err := Activate(context.Background(), Config{Endpoint: "https://dns.example/dns-query"}, hooks)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Refresh(context.Background(), []netip.Addr{netip.MustParseAddr("192.0.2.2")}); err != nil {
		t.Fatal(err)
	}
	if swaps != 1 {
		t.Fatalf("proxy swaps = %d, want 1", swaps)
	}
	if !reflect.DeepEqual(added, []string{"192.0.2.1/32", "192.0.2.2/32"}) {
		t.Fatalf("added routes = %v", added)
	}
	if !reflect.DeepEqual(deleted, []string{"192.0.2.1/32"}) {
		t.Fatalf("deleted routes = %v", deleted)
	}
	_ = s.Close()
}

func TestFinalizeFailureRollsBackDNSProxyAndRoutes(t *testing.T) {
	var restored, closed, deleted bool
	hooks := Hooks{
		Bootstrap: func(context.Context, string) ([]netip.Addr, error) {
			return []netip.Addr{netip.MustParseAddr("192.0.2.10")}, nil
		},
		AddRoute: func(netip.Prefix) (bool, error) { return true, nil },
		DelRoute: func(netip.Prefix) error { deleted = true; return nil },
		Verify:   func(context.Context, string, []netip.Addr) error { return nil },
		StartProxy: func(context.Context, string, []netip.Addr) (io.Closer, error) {
			return closeFunc(func() error { closed = true; return nil }), nil
		},
		SetDNS: func() (func() error, error) {
			return func() error { restored = true; return nil }, nil
		},
		Finalize: func() error { return errors.New("firewall update failed") },
	}
	if _, err := Activate(context.Background(), Config{Endpoint: "https://dns.example/dns-query"}, hooks); err == nil {
		t.Fatal("finalize failure unexpectedly succeeded")
	}
	if !restored || !closed || !deleted {
		t.Fatalf("rollback state restored=%v closed=%v deleted=%v", restored, closed, deleted)
	}
}

func TestRefreshRouteAddFailureKeepsOldState(t *testing.T) {
	oldPrefix := netip.MustParsePrefix("192.0.2.1/32")
	newPrefix := netip.MustParsePrefix("192.0.2.2/32")
	var swaps int
	hooks := Hooks{
		Bootstrap: func(context.Context, string) ([]netip.Addr, error) { return []netip.Addr{oldPrefix.Addr()}, nil },
		AddRoute: func(prefix netip.Prefix) (bool, error) {
			if prefix == newPrefix {
				return false, errors.New("injected route add failure")
			}
			return true, nil
		},
		DelRoute: func(netip.Prefix) error { return nil },
		Verify:   func(context.Context, string, []netip.Addr) error { return nil },
		StartProxy: func(context.Context, string, []netip.Addr) (io.Closer, error) {
			return closeFunc(func() error { return nil }), nil
		},
		ReplaceProxy: func(context.Context, string, []netip.Addr, io.Closer) error {
			swaps++
			return nil
		},
		SetDNS: func() (func() error, error) { return func() error { return nil }, nil },
	}
	s, err := Activate(context.Background(), Config{Endpoint: "https://dns.example/dns-query"}, hooks)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Refresh(context.Background(), []netip.Addr{newPrefix.Addr()}); err == nil {
		t.Fatal("route add failure unexpectedly succeeded")
	}
	if swaps != 0 || !reflect.DeepEqual(s.Routes(), []netip.Prefix{oldPrefix}) {
		t.Fatalf("failed refresh changed state: swaps=%d routes=%v", swaps, s.Routes())
	}
	_ = s.Close()
}

func TestRefreshVerificationAndProxyFailuresKeepOldState(t *testing.T) {
	oldPrefix := netip.MustParsePrefix("192.0.2.1/32")
	newPrefix := netip.MustParsePrefix("192.0.2.2/32")
	verifyErr := errors.New("injected verification failure")
	proxyErr := errors.New("injected proxy swap failure")
	var failVerify, swaps int
	hooks := Hooks{
		Bootstrap: func(context.Context, string) ([]netip.Addr, error) { return []netip.Addr{oldPrefix.Addr()}, nil },
		AddRoute:  func(netip.Prefix) (bool, error) { return true, nil },
		DelRoute:  func(netip.Prefix) error { return nil },
		Verify: func(context.Context, string, []netip.Addr) error {
			if failVerify != 0 {
				return verifyErr
			}
			return nil
		},
		StartProxy: func(context.Context, string, []netip.Addr) (io.Closer, error) {
			return closeFunc(func() error { return nil }), nil
		},
		ReplaceProxy: func(context.Context, string, []netip.Addr, io.Closer) error {
			swaps++
			return proxyErr
		},
		SetDNS: func() (func() error, error) { return func() error { return nil }, nil },
	}
	s, err := Activate(context.Background(), Config{Endpoint: "https://dns.example/dns-query"}, hooks)
	if err != nil {
		t.Fatal(err)
	}
	failVerify = 1
	if err := s.Refresh(context.Background(), []netip.Addr{newPrefix.Addr()}); err == nil || !errors.Is(err, verifyErr) {
		t.Fatalf("verification error = %v", err)
	}
	if swaps != 0 || !reflect.DeepEqual(s.Routes(), []netip.Prefix{oldPrefix}) {
		t.Fatalf("verification failure changed state: swaps=%d routes=%v", swaps, s.Routes())
	}
	failVerify = 0
	if err := s.Refresh(context.Background(), []netip.Addr{newPrefix.Addr()}); err == nil || !errors.Is(err, proxyErr) {
		t.Fatalf("proxy error = %v", err)
	}
	if swaps != 1 || !reflect.DeepEqual(s.Routes(), []netip.Prefix{oldPrefix}) {
		t.Fatalf("proxy failure changed state: swaps=%d routes=%v", swaps, s.Routes())
	}
	_ = s.Close()
}

func TestRefreshVerificationRollbackRetainsRouteForCloseRetry(t *testing.T) {
	oldPrefix := netip.MustParsePrefix("192.0.2.11/32")
	newPrefix := netip.MustParsePrefix("192.0.2.12/32")
	verifyErr := errors.New("verification failed")
	cleanupErr := errors.New("rollback cleanup failed")
	verifyRefresh := false
	cleanupAttempts := 0
	hooks := Hooks{
		Bootstrap: func(context.Context, string) ([]netip.Addr, error) { return []netip.Addr{oldPrefix.Addr()}, nil },
		AddRoute:  func(netip.Prefix) (bool, error) { return true, nil },
		DelRoute: func(prefix netip.Prefix) error {
			if prefix == newPrefix {
				cleanupAttempts++
				if cleanupAttempts == 1 {
					return cleanupErr
				}
			}
			return nil
		},
		Verify: func(context.Context, string, []netip.Addr) error {
			if verifyRefresh {
				return verifyErr
			}
			return nil
		},
		StartProxy: func(context.Context, string, []netip.Addr) (io.Closer, error) {
			return closeFunc(func() error { return nil }), nil
		},
		ReplaceProxy: func(context.Context, string, []netip.Addr, io.Closer) error { return nil },
		SetDNS:       func() (func() error, error) { return func() error { return nil }, nil },
	}
	s, err := Activate(context.Background(), Config{Endpoint: "https://dns.example/dns-query"}, hooks)
	if err != nil {
		t.Fatal(err)
	}
	verifyRefresh = true
	err = s.Refresh(context.Background(), []netip.Addr{newPrefix.Addr()})
	if !errors.Is(err, verifyErr) || !errors.Is(err, cleanupErr) {
		t.Fatalf("refresh error = %v, want verification and cleanup errors", err)
	}
	if got := s.Routes(); !reflect.DeepEqual(got, []netip.Prefix{oldPrefix, newPrefix}) {
		t.Fatalf("routes after rollback cleanup failure = %v", got)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if cleanupAttempts != 2 {
		t.Fatalf("cleanup attempts = %d, want retry during Close", cleanupAttempts)
	}
}

func TestRefreshProxyRollbackRetainsRouteForCloseRetry(t *testing.T) {
	oldPrefix := netip.MustParsePrefix("192.0.2.13/32")
	newPrefix := netip.MustParsePrefix("192.0.2.14/32")
	proxyErr := errors.New("proxy replacement failed")
	cleanupErr := errors.New("rollback cleanup failed")
	cleanupAttempts := 0
	hooks := Hooks{
		Bootstrap: func(context.Context, string) ([]netip.Addr, error) { return []netip.Addr{oldPrefix.Addr()}, nil },
		AddRoute:  func(netip.Prefix) (bool, error) { return true, nil },
		DelRoute: func(prefix netip.Prefix) error {
			if prefix == newPrefix {
				cleanupAttempts++
				if cleanupAttempts == 1 {
					return cleanupErr
				}
			}
			return nil
		},
		Verify: func(context.Context, string, []netip.Addr) error { return nil },
		StartProxy: func(context.Context, string, []netip.Addr) (io.Closer, error) {
			return closeFunc(func() error { return nil }), nil
		},
		ReplaceProxy: func(context.Context, string, []netip.Addr, io.Closer) error { return proxyErr },
		SetDNS:       func() (func() error, error) { return func() error { return nil }, nil },
	}
	s, err := Activate(context.Background(), Config{Endpoint: "https://dns.example/dns-query"}, hooks)
	if err != nil {
		t.Fatal(err)
	}
	err = s.Refresh(context.Background(), []netip.Addr{newPrefix.Addr()})
	if !errors.Is(err, proxyErr) || !errors.Is(err, cleanupErr) {
		t.Fatalf("refresh error = %v, want proxy and cleanup errors", err)
	}
	if got := s.Routes(); !reflect.DeepEqual(got, []netip.Prefix{oldPrefix, newPrefix}) {
		t.Fatalf("routes after rollback cleanup failure = %v", got)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if cleanupAttempts != 2 {
		t.Fatalf("cleanup attempts = %d, want retry during Close", cleanupAttempts)
	}
}

func TestRefreshStaleCleanupFailureCommitsConsistentOwnership(t *testing.T) {
	oldPrefix := netip.MustParsePrefix("192.0.2.1/32")
	newPrefix := netip.MustParsePrefix("192.0.2.2/32")
	removeErr := errors.New("injected stale route cleanup failure")
	var failRemoval bool
	hooks := Hooks{
		Bootstrap: func(context.Context, string) ([]netip.Addr, error) { return []netip.Addr{oldPrefix.Addr()}, nil },
		AddRoute:  func(netip.Prefix) (bool, error) { return true, nil },
		DelRoute: func(prefix netip.Prefix) error {
			if failRemoval && prefix == oldPrefix {
				return removeErr
			}
			return nil
		},
		Verify: func(context.Context, string, []netip.Addr) error { return nil },
		StartProxy: func(context.Context, string, []netip.Addr) (io.Closer, error) {
			return closeFunc(func() error { return nil }), nil
		},
		ReplaceProxy: func(context.Context, string, []netip.Addr, io.Closer) error { return nil },
		SetDNS:       func() (func() error, error) { return func() error { return nil }, nil },
	}
	s, err := Activate(context.Background(), Config{Endpoint: "https://dns.example/dns-query"}, hooks)
	if err != nil {
		t.Fatal(err)
	}
	failRemoval = true
	if err := s.Refresh(context.Background(), []netip.Addr{newPrefix.Addr()}); err == nil || !errors.Is(err, removeErr) {
		t.Fatalf("cleanup error = %v", err)
	}
	if got := s.Routes(); !reflect.DeepEqual(got, []netip.Prefix{newPrefix, oldPrefix}) {
		t.Fatalf("routes after cleanup failure = %v", got)
	}
	failRemoval = false
	if err := s.Refresh(context.Background(), []netip.Addr{newPrefix.Addr()}); err != nil {
		t.Fatal(err)
	}
	if got := s.Routes(); !reflect.DeepEqual(got, []netip.Prefix{newPrefix}) {
		t.Fatalf("routes after retry = %v", got)
	}
	_ = s.Close()
}

func TestRecoverReappliesRuntimeState(t *testing.T) {
	endpoint := "https://dns.example/dns-query"
	address := netip.MustParseAddr("192.0.2.1")
	var events []string
	hooks := Hooks{
		Bootstrap: func(context.Context, string) ([]netip.Addr, error) { return []netip.Addr{address}, nil },
		AddRoute: func(prefix netip.Prefix) (bool, error) {
			events = append(events, "route:"+prefix.String())
			return true, nil
		},
		DelRoute: func(netip.Prefix) error { return nil },
		Verify: func(context.Context, string, []netip.Addr) error {
			events = append(events, "verify")
			return nil
		},
		StartProxy: func(context.Context, string, []netip.Addr) (io.Closer, error) {
			return closeFunc(func() error { return nil }), nil
		},
		ReplaceProxy: func(context.Context, string, []netip.Addr, io.Closer) error {
			events = append(events, "proxy")
			return nil
		},
		SetDNS: func() (func() error, error) { return func() error { return nil }, nil },
		ReapplyDNS: func() error {
			events = append(events, "dns")
			return nil
		},
		Finalize: func() error {
			events = append(events, "finalize")
			return nil
		},
	}
	s, err := Activate(context.Background(), Config{Endpoint: endpoint}, hooks)
	if err != nil {
		t.Fatal(err)
	}
	events = nil
	if err := s.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if want := []string{"route:192.0.2.1/32", "verify", "proxy", "dns", "finalize"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("recovery events = %v, want %v", events, want)
	}
	_ = s.Close()
}
