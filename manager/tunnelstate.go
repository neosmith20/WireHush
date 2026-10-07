/* SPDX-License-Identifier: MIT
 * Copyright (C) 2019-2026 WireGuard LLC. All Rights Reserved.
 */
package manager

type TunnelState int

const (
	TunnelUnknown TunnelState = iota
	TunnelStarted
	TunnelStopped
	TunnelStarting
	TunnelStopping
)
