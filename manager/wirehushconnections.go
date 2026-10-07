/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */
package manager

import (
	"net"
	"sync"
)

const wireHushMaximumConnections = 32

type wireHushBoundedListener struct {
	net.Listener
	slots    chan struct{}
	closed   chan struct{}
	once     sync.Once
	closeErr error
}

func boundedWireHushListener(listener net.Listener, maximum int) *wireHushBoundedListener {
	return &wireHushBoundedListener{Listener: listener, slots: make(chan struct{}, maximum), closed: make(chan struct{})}
}
func (listener *wireHushBoundedListener) Accept() (net.Conn, error) {
	select {
	case listener.slots <- struct{}{}:
	case <-listener.closed:
		return nil, net.ErrClosed
	}
	select {
	case <-listener.closed:
		<-listener.slots
		return nil, net.ErrClosed
	default:
	}
	conn, err := listener.Listener.Accept()
	if err != nil {
		<-listener.slots
		return nil, err
	}
	return &wireHushBoundedConnection{Conn: conn, release: func() { <-listener.slots }}, nil
}
func (listener *wireHushBoundedListener) Close() error {
	listener.once.Do(func() { close(listener.closed); listener.closeErr = listener.Listener.Close() })
	return listener.closeErr
}

type wireHushBoundedConnection struct {
	net.Conn
	once     sync.Once
	release  func()
	closeErr error
}

func (conn *wireHushBoundedConnection) Close() error {
	conn.once.Do(func() { conn.closeErr = conn.Conn.Close(); conn.release() })
	return conn.closeErr
}

// Preserve access to the actual Windows pipe handle for token authentication.
// No client-supplied descriptor or identity is accepted.
func (conn *wireHushBoundedConnection) Fd() uintptr {
	if pipe, ok := conn.Conn.(interface{ Fd() uintptr }); ok {
		return pipe.Fd()
	}
	return 0
}
