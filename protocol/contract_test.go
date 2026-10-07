/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package protocol

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"testing"
)

func TestProtocolVersionRejectsIncompatibleComponents(t *testing.T) {
	for _, request := range []*HandshakeRequest{nil, {}, {ProtocolMajor: Major + 1}} {
		if status.Code(ValidateVersion(request)) != codes.FailedPrecondition {
			t.Fatalf("incompatible version accepted: %+v", request)
		}
	}
	if err := ValidateVersion(&HandshakeRequest{ProtocolMajor: Major, ProtocolMinor: 100}); err != nil {
		t.Fatal(err)
	}
}

func TestProtocolSnapshotPreservesUnknownRuntimeValues(t *testing.T) {
	input := &SnapshotReply{InstanceId: "test", Revision: 4, Tunnels: []*TunnelSnapshot{{Tunnel: &TunnelRef{TunnelId: "test", Scope: Scope_SCOPE_PRIVATE}, State: TunnelState_TUNNEL_STATE_UNKNOWN}}}
	data, err := proto.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var output SnapshotReply
	if err := proto.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(input, &output) || output.Tunnels[0].RxBytes != nil || output.Tunnels[0].LatestHandshakeUnix != nil || output.Tunnels[0].DnsReady != nil {
		t.Fatal("unknown runtime evidence became invented values")
	}
}

func TestProtocolRequestsCannotAssertWindowsIdentity(t *testing.T) {
	for _, message := range []proto.Message{&HandshakeRequest{}, &TunnelRef{}, &CreateTunnelRequest{}, &UpdateTunnelRequest{}, &BootstrapSettings{}} {
		fields := message.ProtoReflect().Descriptor().Fields()
		for i := 0; i < fields.Len(); i++ {
			switch string(fields.Get(i).Name()) {
			case "owner_sid", "sid", "session", "administrator", "path", "wirehush_user":
				t.Fatalf("client-authoritative security field in %T", message)
			}
		}
	}
}
