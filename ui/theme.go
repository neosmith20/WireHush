//go:build windows

/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush contributors. All Rights Reserved.
 */

package ui

import (
	"github.com/lxn/walk"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
)

// The manager deliberately owns its presentation rather than inheriting the
// bright legacy control palette. These are calm, high-contrast surfaces that
// keep the cyan brand color and green connection state easy to scan.
var (
	uiCanvasBrush  = mustBrush(walk.RGB(10, 18, 25))
	uiRailBrush    = mustBrush(walk.RGB(12, 24, 34))
	uiCardBrush    = mustBrush(walk.RGB(17, 31, 42))
	uiHeaderBrush  = mustBrush(walk.RGB(14, 27, 38))
	uiAccentBrush  = mustBrush(walk.RGB(0, 184, 224))
	uiTextColor    = walk.RGB(232, 241, 248)
	uiMutedColor   = walk.RGB(165, 185, 200)
	uiAccentColor  = walk.RGB(50, 203, 239)
	uiHealthyColor = walk.RGB(58, 221, 126)
)

func mustBrush(color walk.Color) walk.Brush {
	brush, err := walk.NewSolidColorBrush(color)
	if err != nil {
		panic(err)
	}
	return brush
}

type textColorSetter interface {
	SetTextColor(walk.Color)
}

func applyDarkSurface(widget walk.Widget, brush walk.Brush) {
	widget.SetBackground(brush)
	if text, ok := widget.(textColorSetter); ok {
		text.SetTextColor(uiTextColor)
	}
}

func applyMutedText(widget walk.Widget) {
	if text, ok := widget.(textColorSetter); ok {
		text.SetTextColor(uiMutedColor)
	}
}

func applyDarkWindow(hwnd win.HWND) {
	// DarkMode_Explorer gives standard controls a native dark treatment on
	// supported Windows releases. The explicit surfaces above remain the
	// fallback for older systems.
	win.SetWindowTheme(hwnd, windows.StringToUTF16Ptr("DarkMode_Explorer"), nil)
}
