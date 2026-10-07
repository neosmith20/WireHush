/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush contributors. All Rights Reserved.
 */

package conf

import (
	"testing"

	"golang.zx2c4.com/wireguard/windows/product"
)

func TestServiceNameOfTunnelUsesActiveProductIdentity(t *testing.T) {
	serviceName, err := ServiceNameOfTunnel("Example")
	if err != nil {
		t.Fatalf("ServiceNameOfTunnel returned an error: %v", err)
	}
	if serviceName != "WireHushTunnel$Example" {
		t.Fatalf("service name = %q, want %q", serviceName, "WireHushTunnel$Example")
	}
	if serviceName != product.TunnelServicePrefix+"Example" {
		t.Fatal("service name does not use the active product tunnel prefix")
	}
	if serviceName == "TunnelMintTunnel$Example" {
		t.Fatal("service name uses the legacy TunnelMint tunnel prefix")
	}
}
