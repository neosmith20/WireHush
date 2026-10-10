/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2019-2026 WireGuard LLC. All Rights Reserved.
 */

package version

const (
	Number = "0.1.0"
)

// SourceCommit is supplied by the reproducible V1 build. Unstamped local
// builds identify themselves explicitly rather than claiming release provenance.
var SourceCommit = "unstamped"

func Provenance() string { return Number + "+" + SourceCommit }
