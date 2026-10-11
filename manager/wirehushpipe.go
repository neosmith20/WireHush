/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package manager

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"runtime"
	"time"
	"unsafe"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/windows/elevate"
	"golang.zx2c4.com/wireguard/windows/protocol"
	"google.golang.org/grpc/credentials"
)

const wireHushUsersGroup = "WireHush Users"

// Excludes FILE_CREATE_PIPE_INSTANCE (0x4), unlike GENERIC_WRITE. Clients must
// open with these precise rights rather than requesting generic read/write.
const wireHushPipeClientAccess uint32 = 0x12019b
const wireHushHTTP2Preface = "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"

var impersonateWireHushPipe = windows.NewLazySystemDLL("advapi32.dll").NewProc("ImpersonateNamedPipeClient")
var wireHushPipeClientSession = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetNamedPipeClientSessionId")

type wireHushPipeIdentity struct {
	credentials.CommonAuthInfo
	Caller    wireHushCaller
	SessionID uint32
}

func (wireHushPipeIdentity) AuthType() string { return "wirehush.windows-pipe" }

func validateWireHushPipeIdentity(identity wireHushPipeIdentity) error {
	if err := identity.Caller.Validate(); err != nil {
		return errWireHushAccessDenied
	}
	if identity.Caller.SID == "S-1-5-7" || identity.Caller.SID == "S-1-5-2" {
		return errWireHushAccessDenied
	}
	return nil
}

func wireHushPipeSecurityDescriptor(group *windows.SID) (string, error) {
	if group == nil || !group.IsValid() {
		return "", errWireHushAccessDenied
	}
	// Authenticated local users may manage their own private namespace. Network
	// and anonymous logons remain denied; shared permissions are checked per RPC.
	return "O:SYG:SYD:P(D;;GA;;;AN)(D;;GA;;;NU)(A;;GA;;;SY)(A;;0x12019b;;;AU)(A;;0x12019b;;;BA)(A;;0x12019b;;;" + group.String() + ")", nil
}

func listenWireHushPipe() (net.Listener, *windows.SID, error) {
	computer, err := os.Hostname()
	if err != nil {
		return nil, nil, err
	}
	group, _, kind, err := windows.LookupSID("", computer+"\\"+wireHushUsersGroup)
	if err != nil {
		return nil, nil, err
	}
	if kind != windows.SidTypeAlias {
		return nil, nil, errWireHushAccessDenied
	}
	sddl, err := wireHushPipeSecurityDescriptor(group)
	if err != nil {
		return nil, nil, err
	}
	listener, err := winio.ListenPipe(protocol.PipePath, &winio.PipeConfig{SecurityDescriptor: sddl, InputBufferSize: 65536, OutputBufferSize: 65536})
	if err != nil {
		return nil, nil, err
	}
	return boundedWireHushListener(listener, wireHushMaximumConnections), group, nil
}

type wireHushPipeCredentials struct{ group *windows.SID }

func (c wireHushPipeCredentials) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{SecurityProtocol: "windows-named-pipe", SecurityVersion: "1"}
}
func (c wireHushPipeCredentials) Clone() credentials.TransportCredentials {
	return wireHushPipeCredentials{group: c.group}
}
func (wireHushPipeCredentials) OverrideServerName(string) error {
	return errors.New("named-pipe identity cannot be overridden")
}
func (wireHushPipeCredentials) ClientHandshake(context.Context, string, net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return nil, nil, errors.New("server-only named-pipe credentials")
}

type wireHushPrefacedConn struct {
	net.Conn
	reader io.Reader
}

func (c *wireHushPrefacedConn) Read(p []byte) (int, error) { return c.reader.Read(p) }

func (c wireHushPipeCredentials) ServerHandshake(conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	if err := conn.SetDeadline(time.Now().Add(protocol.PipeTimeout)); err != nil {
		return nil, nil, err
	}
	defer conn.SetDeadline(time.Time{})
	// Reading actual client data establishes the Windows security context used
	// by ImpersonateNamedPipeClient. Replay the standard HTTP/2 preface for gRPC;
	// no custom application framing or identity assertion is introduced.
	preface := make([]byte, len(wireHushHTTP2Preface))
	if _, err := io.ReadFull(conn, preface); err != nil {
		return nil, nil, err
	}
	if string(preface) != wireHushHTTP2Preface {
		return nil, nil, errWireHushAccessDenied
	}
	identity, err := authenticateWireHushPipe(conn, c.group)
	if err != nil {
		return nil, nil, err
	}
	return &wireHushPrefacedConn{Conn: conn, reader: io.MultiReader(bytes.NewReader(preface), conn)}, identity, nil
}

func authenticateWireHushPipe(conn net.Conn, group *windows.SID) (wireHushPipeIdentity, error) {
	fd, ok := conn.(interface{ Fd() uintptr })
	if !ok || group == nil {
		return wireHushPipeIdentity{}, errWireHushAccessDenied
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	result, _, err := impersonateWireHushPipe.Call(fd.Fd())
	if result == 0 {
		return wireHushPipeIdentity{}, err
	}
	defer func() {
		if err := windows.RevertToSelf(); err != nil {
			// Continuing under a client security context is unsafe. Process exit
			// preserves independent tunnel services, matching manager-crash rules.
			log.Print("WireHush pipe authentication could not restore service identity")
			os.Exit(1)
		}
	}()
	var token windows.Token
	if err := windows.OpenThreadToken(windows.CurrentThread(), windows.TOKEN_QUERY|windows.TOKEN_DUPLICATE, true, &token); err != nil {
		return wireHushPipeIdentity{}, err
	}
	defer token.Close()
	for _, check := range []struct {
		kind     windows.WELL_KNOWN_SID_TYPE
		required bool
	}{{windows.WinAuthenticatedUserSid, true}, {windows.WinNetworkSid, false}, {windows.WinAnonymousSid, false}} {
		sid, err := windows.CreateWellKnownSid(check.kind)
		if err != nil {
			return wireHushPipeIdentity{}, err
		}
		member, err := token.IsMember(sid)
		if err != nil || member != check.required {
			return wireHushPipeIdentity{}, errWireHushAccessDenied
		}
	}
	user, err := token.GetTokenUser()
	if err != nil {
		return wireHushPipeIdentity{}, err
	}
	var session, size uint32
	if err := windows.GetTokenInformation(token, windows.TokenSessionId, (*byte)(unsafe.Pointer(&session)), uint32(unsafe.Sizeof(session)), &size); err != nil {
		return wireHushPipeIdentity{}, err
	}
	var pipeSession uint32
	result, _, err = wireHushPipeClientSession.Call(fd.Fd(), uintptr(unsafe.Pointer(&pipeSession)))
	if result == 0 {
		return wireHushPipeIdentity{}, err
	}
	if pipeSession != session {
		return wireHushPipeIdentity{}, errWireHushAccessDenied
	}
	member, err := token.IsMember(group)
	if err != nil {
		return wireHushPipeIdentity{}, err
	}
	identity := wireHushPipeIdentity{
		CommonAuthInfo: credentials.CommonAuthInfo{SecurityLevel: credentials.PrivacyAndIntegrity},
		Caller:         wireHushCaller{SID: user.User.Sid.String(), Administrator: elevate.TokenIsElevatedOrElevatable(token), WireHushUser: member},
		SessionID:      session,
	}
	if err := validateWireHushPipeIdentity(identity); err != nil {
		return wireHushPipeIdentity{}, err
	}
	return identity, nil
}
