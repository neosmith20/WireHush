/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */
package ringlogger

import (
	"io"
	"log"
	"os"
	"strings"
)

type wireHushSanitizedWriter struct{ destination io.Writer }

func (writer wireHushSanitizedWriter) Write(data []byte) (int, error) {
	category := wireHushLogCategory(string(data))
	_, err := io.WriteString(writer.destination, category+"\n")
	return len(data), err
}
func wireHushLogCategory(message string) string {
	// Never copy caller text, driver text, config names, keys, URL paths or raw
	// error messages. Fixed categories retain useful lifecycle diagnostics.
	message = strings.TrimSpace(message)
	if strings.HasPrefix(message, "[") {
		if index := strings.Index(message, "] "); index >= 0 {
			message = message[index+2:]
		}
	}
	switch message {
	case "manager-ready", "manager-idle-exit", "manager-session-exit", "manager-stop-failed", "manager-transport-failed":
		return message
	case "Startup complete":
		return "tunnel-ready"
	case "Shutting down":
		return "tunnel-worker-ending"
	case "Starting encrypted DNS runtime":
		return "encrypted-dns-starting"
	case "Watching network interfaces":
		return "network-monitor-starting"
	}
	lower := strings.ToLower(message)
	if strings.Contains(lower, "failed") || strings.Contains(lower, "unable") || strings.Contains(lower, "error") {
		return "backend-operation-failed"
	}
	return "backend-event-redacted"
}
func InitWireHushLogger(file *os.File, tag string) error {
	if Global != nil {
		file.Close()
		return nil
	}
	logger, err := newRingloggerFromFile(file, tag)
	if err != nil {
		file.Close()
		return err
	}
	Global = logger
	log.SetFlags(0)
	log.SetOutput(wireHushSanitizedWriter{destination: logger})
	// Do not install the inherited runtime stack-output override: stack traces
	// and panic payloads are not a safe user-facing diagnostic format.
	return nil
}
