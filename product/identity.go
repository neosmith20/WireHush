/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush contributors. All Rights Reserved.
 */

// Package product contains the WireHush product identity. Persistent values
// marked Legacy are intentionally retained so an installed TunnelMint build
// upgrades without losing tunnel configurations or colliding with WireGuard.
package product

const (
	Name = "WireHush"

	// These are stable Windows service keys, not user-facing names.
	LegacyManagerServiceName  = "TunnelMintManager"
	LegacyTunnelServicePrefix = "TunnelMintTunnel$"
	LegacyDataDirectoryName   = "TunnelMint"
	LegacyAdminRegistryKey    = `Software\TunnelMint`

	ManagerServiceName         = "WireHushManager"
	ManagerServiceDisplayName  = "WireHush Manager"
	TunnelServicePrefix        = "WireHushTunnel$"
	TunnelServiceDisplayPrefix = "WireHush Tunnel: "

	DataDirectoryName = LegacyDataDirectoryName
	AdminRegistryKey  = LegacyAdminRegistryKey

	ManagerWindowClass = "WireHush UI - Manage Tunnels"
	ManagerWindowTitle = "WireHush"

	UserAgentName = "WireHush"
)
