/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 TunnelMint contributors. All Rights Reserved.
 */

package doh

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

var testDNSQuery = []byte{0x12, 0x34, 0x01, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e', 0x00, 0x00, 0x01, 0x00, 0x01}
var testDNSResponse = []byte{0x12, 0x34, 0x81, 0x80, 0x00, 0x01, 0x00, 0x01}

type testTimeoutError struct{}

func (testTimeoutError) Error() string   { return "timed out" }
func (testTimeoutError) Timeout() bool   { return true }
func (testTimeoutError) Temporary() bool { return true }

func newTLSServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *http.Client) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	return server, server.Client()
}

func TestQueryPOSTHeadersAndPath(t *testing.T) {
	const path = "/dns-query/provider/client-id"
	server, client := newTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != path {
			t.Errorf("path = %q, want %q", r.URL.Path, path)
		}
		if got := r.Header.Get("Content-Type"); got != "application/dns-message" {
			t.Errorf("Content-Type = %q", got)
		}
		if got := r.Header.Get("Accept"); got != "application/dns-message" {
			t.Errorf("Accept = %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		if string(body) != string(testDNSQuery) {
			t.Errorf("query body = %x, want %x", body, testDNSQuery)
		}
		w.Header().Set("Content-Type", "application/dns-message")
		_, _ = w.Write(testDNSResponse)
	})

	doh, err := NewClient(server.URL+path, Options{HTTPClient: client})
	if err != nil {
		t.Fatal(err)
	}
	response, err := doh.Query(context.Background(), testDNSQuery)
	if err != nil {
		t.Fatal(err)
	}
	if string(response) != string(testDNSResponse) {
		t.Errorf("response = %x, want %x", response, testDNSResponse)
	}
}

func TestQueryRejectsHTTPStatusAndContentType(t *testing.T) {
	for _, test := range []struct {
		name        string
		status      int
		contentType string
	}{
		{name: "status", status: http.StatusBadGateway, contentType: "application/dns-message"},
		{name: "content type", status: http.StatusOK, contentType: "text/plain"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server, client := newTLSServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", test.contentType)
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte("not a DNS response"))
			})
			doh, err := NewClient(server.URL, Options{HTTPClient: client})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := doh.Query(context.Background(), testDNSQuery); err == nil {
				t.Fatal("expected query to fail")
			}
		})
	}
}

func TestQueryTimeoutAndCancellation(t *testing.T) {
	server, client := newTLSServer(t, func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Header().Set("Content-Type", "application/dns-message")
		_, _ = w.Write(testDNSResponse)
	})
	doh, err := NewClient(server.URL, Options{HTTPClient: client, Timeout: 25 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	_, err = doh.Query(context.Background(), testDNSQuery)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = doh.Query(ctx, testDNSQuery)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
}

func TestDialBootstrappedAddressesFallsBackWithinRequestDeadline(t *testing.T) {
	first := netip.MustParseAddr("2001:db8::1")
	second := netip.MustParseAddr("192.0.2.1")
	// Keep this comfortably above Windows timer granularity and scheduler jitter.
	// The invariant under test is per-candidate deadline partitioning, not a 100 ms wall clock.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	var calls []netip.Addr
	conn, err := dialBootstrappedAddresses(ctx, "tcp", "443", []netip.Addr{first, second}, func(ctx context.Context, _ string, address string) (net.Conn, error) {
		host, _, splitErr := net.SplitHostPort(address)
		if splitErr != nil {
			return nil, splitErr
		}
		candidate, parseErr := netip.ParseAddr(host)
		if parseErr != nil {
			return nil, parseErr
		}
		calls = append(calls, candidate)
		if candidate == first {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		client, server := net.Pipe()
		server.Close()
		return client, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if len(calls) != 2 || calls[0] != first || calls[1] != second {
		t.Fatalf("dial candidates = %v, want [%v %v]", calls, first, second)
	}
}

func TestDoHTransportDiagnosticCategoriesAvoidAddressData(t *testing.T) {
	if got := dohAddressFamily(netip.MustParseAddr("192.0.2.1")); got != "IPv4" {
		t.Fatalf("IPv4 family = %q", got)
	}
	if got := dohAddressFamily(netip.MustParseAddr("2001:db8::1")); got != "IPv6" {
		t.Fatalf("IPv6 family = %q", got)
	}
	for _, test := range []struct {
		err  error
		want string
	}{
		{context.DeadlineExceeded, "timeout"},
		{context.Canceled, "canceled"},
		{testTimeoutError{}, "timeout"},
		{errors.New("unreachable"), "network-error"},
	} {
		if got := dohDialFailureCategory(test.err); got != test.want {
			t.Errorf("failure category for %v = %q, want %q", test.err, got, test.want)
		}
	}
}

func TestQueryRejectsUntrustedTLS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/dns-message")
		_, _ = w.Write(testDNSResponse)
	}))
	defer server.Close()
	doh, err := NewClient(server.URL, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := doh.Query(context.Background(), testDNSQuery); err == nil {
		t.Fatal("expected untrusted TLS certificate to fail")
	}
}

func TestQueryRejectsHTTPSRedirect(t *testing.T) {
	plaintext := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/dns-message")
		_, _ = w.Write(testDNSResponse)
	}))
	defer plaintext.Close()
	tlsServer, client := newTLSServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", plaintext.URL)
		w.WriteHeader(http.StatusFound)
	})
	doh, err := NewClient(tlsServer.URL, Options{HTTPClient: client})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := doh.Query(context.Background(), testDNSQuery); err == nil || !strings.Contains(err.Error(), "HTTP status 302") {
		t.Fatalf("redirect error = %v", err)
	}
}

func TestQueryRejectsOversizedResponse(t *testing.T) {
	server, client := newTLSServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/dns-message")
		_, _ = w.Write([]byte("0123456789"))
	})
	doh, err := NewClient(server.URL, Options{HTTPClient: client, MaxResponseSize: 4})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := doh.Query(context.Background(), testDNSQuery); err == nil || !strings.Contains(err.Error(), "exceeds 4 bytes") {
		t.Fatalf("oversized response error = %v", err)
	}
}

func TestNewClientValidatesEndpoint(t *testing.T) {
	for _, endpoint := range []string{"http://dns.example.com/dns-query", "https://", "https://:443/dns-query"} {
		if _, err := NewClient(endpoint, Options{}); err == nil {
			t.Errorf("NewClient(%q) unexpectedly succeeded", endpoint)
		}
	}
}
