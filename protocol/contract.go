/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package protocol

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"time"
)

const (
	Major               uint32 = 1
	Minor               uint32 = 0
	MaximumMessageBytes        = 1 << 20
	RequestTimeout             = 10 * time.Second
	PipeTimeout                = 5 * time.Second
	PipePath                   = `\\.\pipe\WireHush.Manager.v1`
)

func ValidateVersion(request *HandshakeRequest) error {
	if request == nil || request.ProtocolMajor != Major {
		return status.Error(codes.FailedPrecondition, "WireHush components require repair or update")
	}
	return nil
}
