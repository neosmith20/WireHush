//go:build !wirehush_v1

/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2019-2026 WireGuard LLC. All Rights Reserved.
 */

package manager

import (
	"log"
)

type UpdateState uint32

const (
	UpdateStateUnknown UpdateState = iota
	UpdateStateFoundUpdate
	UpdateStateUpdatesDisabledUnofficialBuild
)

var updateState = UpdateStateUnknown

func checkForUpdates() {
	// WireHush has no signed update infrastructure yet. Keep the upstream
	// updater package available for a future implementation, but never contact
	// the upstream WireGuard update service from the runtime.
	log.Println("WireHush automatic updates are disabled")
	updateState = UpdateStateUpdatesDisabledUnofficialBuild
	IPCServerNotifyUpdateFound(updateState)
}
