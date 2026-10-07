/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */
package ringlogger

import (
	"bytes"
	"strings"
	"testing"
)

func TestWireHushLogsNeverCopySensitiveInput(t *testing.T) {
	var output bytes.Buffer
	writer := wireHushSanitizedWriter{destination: &output}
	for _, input := range []string{"PrivateKey = SECRET", "[SECRET-TUNNEL] Startup complete", "unable to open C:\\SECRET\\record: S-1-5-21-SECRET", "https://user:SECRET@dns.example/SECRET?credential=SECRET", "error PrivateKey=SECRET\nmanager-ready"} {
		n, err := writer.Write([]byte(input))
		if err != nil || n != len(input) {
			t.Fatal(err)
		}
	}
	for _, forbidden := range []string{"SECRET", "dns.example", "PrivateKey", "S-1-5-21", "C:"} {
		if strings.Contains(output.String(), forbidden) {
			t.Fatal("sensitive diagnostic data escaped redaction")
		}
	}
	if wireHushLogCategory("manager-ready") != "manager-ready" || wireHushLogCategory("[private name] Startup complete") != "tunnel-ready" {
		t.Fatal("fixed lifecycle categories were lost")
	}
}
