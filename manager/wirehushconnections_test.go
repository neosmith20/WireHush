/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */
package manager

import (
	"net"
	"sync"
	"testing"
	"time"
)

type wireHushFixtureListener struct {
	incoming chan net.Conn
	closed   chan struct{}
	once     sync.Once
}

func (listener *wireHushFixtureListener) Accept() (net.Conn, error) {
	select {
	case conn := <-listener.incoming:
		return conn, nil
	case <-listener.closed:
		return nil, net.ErrClosed
	}
}
func (listener *wireHushFixtureListener) Close() error {
	listener.once.Do(func() { close(listener.closed) })
	return nil
}
func (listener *wireHushFixtureListener) Addr() net.Addr { return nil }
func TestWireHushConnectionBoundReleasesExactlyOnceAndCloseUnblocks(t *testing.T) {
	fixture := &wireHushFixtureListener{incoming: make(chan net.Conn, 2), closed: make(chan struct{})}
	listener := boundedWireHushListener(fixture, 1)
	defer listener.Close()
	first, firstPeer := net.Pipe()
	second, secondPeer := net.Pipe()
	defer firstPeer.Close()
	defer secondPeer.Close()
	fixture.incoming <- first
	fixture.incoming <- second
	held, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	accepted := make(chan net.Conn, 1)
	go func() { conn, _ := listener.Accept(); accepted <- conn }()
	select {
	case <-accepted:
		t.Fatal("connection bound exceeded")
	case <-time.After(25 * time.Millisecond):
	}
	held.Close()
	held.Close()
	select {
	case conn := <-accepted:
		if conn == nil {
			t.Fatal("released slot not reusable")
		}
		defer conn.Close()
	case <-time.After(time.Second):
		t.Fatal("release did not unblock acceptance")
	}
	stopped := make(chan error, 1)
	go func() { _, err := listener.Accept(); stopped <- err }()
	listener.Close()
	select {
	case err := <-stopped:
		if err == nil {
			t.Fatal("closed listener accepted connection")
		}
	case <-time.After(time.Second):
		t.Fatal("listener close retained a blocked accept")
	}
}
