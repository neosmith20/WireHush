/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */

package conf

import "errors"

// WireHushAdapterNameOfTunnelID returns the immutable WireHush adapter name for id.
func WireHushAdapterNameOfTunnelID(id TunnelID) (string, error) {
	if !id.Valid() {
		return "", errors.New("TunnelID is not valid")
	}
	return "WireHush-" + id.Token(), nil
}
