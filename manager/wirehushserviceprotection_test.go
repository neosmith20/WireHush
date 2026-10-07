/* SPDX-License-Identifier: MIT
 * Copyright (C) 2026 WireHush. All Rights Reserved.
 */
package manager

import (
	"errors"
	"golang.org/x/sys/windows"
	"reflect"
	"strings"
	"testing"
)

func TestWireHushWorkerNeverPublishesLocatorOrStartsBeforeProtection(t *testing.T) {
	for _, failing := range []string{"protect", "bind", "start", ""} {
		var calls []string
		failure := errors.New("fixture failure")
		stage := func(name string) func() error {
			return func() error {
				calls = append(calls, name)
				if name == failing {
					return failure
				}
				return nil
			}
		}
		err := finishWireHushWorkerProvision(stage("protect"), stage("bind"), stage("start"))
		expected := []string{"protect", "bind", "start"}
		if failing == "protect" {
			expected = expected[:1]
		} else if failing == "bind" {
			expected = expected[:2]
		}
		if !reflect.DeepEqual(calls, expected) || (failing != "" && !errors.Is(err, failure)) || (failing == "" && err != nil) {
			t.Fatalf("stage %q: calls=%v error=%v", failing, calls, err)
		}
	}
	if _, err := windows.SecurityDescriptorFromString(wireHushWorkerServiceSDDL); err != nil {
		t.Fatal(err)
	}
	for _, broad := range []string{";;;WD)", ";;;AU)", ";;;BU)"} {
		if strings.Contains(wireHushWorkerServiceSDDL, broad) {
			t.Fatal("worker permits broad service access")
		}
	}
}
