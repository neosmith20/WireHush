//go:build windows

/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush contributors. All Rights Reserved.
 */

package ui

import (
	"sort"
	"strings"

	"github.com/lxn/walk"
	"golang.zx2c4.com/wireguard/windows/conf"
	"golang.zx2c4.com/wireguard/windows/manager"
)

const railRowHeight = 62

type railItem struct {
	tunnel manager.Tunnel
	state  manager.TunnelState
}

// ConnectionRail is intentionally rendered by WireHush instead of presenting
// the legacy TableView. It remains a small view over the existing manager
// tunnel list and never owns tunnel state.
type ConnectionRail struct {
	*walk.Composite
	search       *walk.LineEdit
	list         *walk.CustomWidget
	items        []railItem
	selectedName string
	filter       string
	onSelected   func(string)
	font         *walk.Font
	boldFont     *walk.Font
	tunnelCB     *manager.TunnelChangeCallback
	tunnelsCB    *manager.TunnelsChangeCallback
}

func NewConnectionRail(parent walk.Container) (*ConnectionRail, error) {
	rail := &ConnectionRail{}
	composite, err := walk.NewComposite(parent)
	if err != nil {
		return nil, err
	}
	rail.Composite = composite
	layout := walk.NewVBoxLayout()
	layout.SetMargins(walk.Margins{18, 18, 18, 18})
	layout.SetSpacing(10)
	rail.SetLayout(layout)
	applyDarkSurface(rail, uiRailBrush)
	rail.SetMinMaxSize(walk.Size{290, 0}, walk.Size{290, 0})

	heading, err := walk.NewLabel(rail)
	if err != nil {
		return nil, err
	}
	heading.SetText("Connections")
	heading.SetTextColor(uiTextColor)
	rail.boldFont, _ = walk.NewFont("Segoe UI Semibold", 14, 0)
	heading.SetFont(rail.boldFont)

	if rail.search, err = walk.NewLineEdit(rail); err != nil {
		return nil, err
	}
	rail.search.SetCueBanner("Search tunnels…")
	rail.search.SetBackground(uiCardBrush)
	rail.search.SetTextColor(uiTextColor)
	applyDarkWindow(rail.search.Handle())
	rail.search.TextChanged().Attach(func() {
		rail.filter = strings.ToLower(strings.TrimSpace(rail.search.Text()))
		rail.Load()
	})

	rail.font, _ = walk.NewFont("Segoe UI", 10, 0)
	rail.list, err = walk.NewCustomWidgetPixels(rail, 0, rail.paint)
	if err != nil {
		return nil, err
	}
	rail.list.SetPaintMode(walk.PaintBuffered)
	rail.list.SetInvalidatesOnResize(true)
	rail.list.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
		if button != walk.LeftButton {
			return
		}
		index := y / rail.list.IntFrom96DPI(railRowHeight)
		if index < 0 || index >= len(rail.items) {
			return
		}
		rail.Select(rail.items[index].tunnel.Name)
	})

	rail.tunnelCB = manager.IPCClientRegisterTunnelChange(func(_ *manager.Tunnel, _ manager.TunnelState, _ manager.TunnelState, _ error) { rail.Load() })
	rail.tunnelsCB = manager.IPCClientRegisterTunnelsChange(rail.Load)
	rail.Load()
	return rail, nil
}

func (rail *ConnectionRail) Dispose() {
	if rail.tunnelCB != nil {
		rail.tunnelCB.Unregister()
	}
	if rail.tunnelsCB != nil {
		rail.tunnelsCB.Unregister()
	}
	rail.Composite.Dispose()
}

func (rail *ConnectionRail) SetSelectionHandler(handler func(string)) { rail.onSelected = handler }

func (rail *ConnectionRail) Select(name string) {
	if rail.selectedName == name {
		return
	}
	rail.selectedName = name
	rail.list.Invalidate()
	if rail.onSelected != nil {
		rail.onSelected(name)
	}
}

func (rail *ConnectionRail) Load() {
	tunnels, err := manager.IPCClientTunnels()
	if err != nil {
		return
	}
	filter := rail.filter
	items := make([]railItem, 0, len(tunnels))
	for _, tunnel := range tunnels {
		if filter != "" && !strings.Contains(strings.ToLower(tunnel.Name), filter) {
			continue
		}
		state, _ := tunnel.State()
		items = append(items, railItem{tunnel: tunnel, state: state})
	}
	sort.SliceStable(items, func(i, j int) bool { return conf.TunnelNameIsLess(items[i].tunnel.Name, items[j].tunnel.Name) })
	rail.Synchronize(func() {
		rail.items = items
		found := false
		for _, item := range items {
			found = found || item.tunnel.Name == rail.selectedName
		}
		if !found {
			rail.selectedName = ""
		}
		rail.list.Invalidate()
	})
}

func (rail *ConnectionRail) paint(canvas *walk.Canvas, bounds walk.Rectangle) error {
	canvas.FillRectangle(uiRailBrush, bounds)
	rowHeight := rail.IntFrom96DPI(railRowHeight)
	selectedBrush := mustBrush(walk.RGB(11, 93, 126))
	defer selectedBrush.Dispose()
	for index, item := range rail.items {
		row := walk.Rectangle{X: bounds.X, Y: bounds.Y + index*rowHeight, Width: bounds.Width, Height: rowHeight}
		if item.tunnel.Name == rail.selectedName {
			canvas.FillRectangle(selectedBrush, row)
		}
		dotColor := uiMutedColor
		statusColor := uiMutedColor
		if item.state == manager.TunnelStarted {
			dotColor, statusColor = uiHealthyColor, uiHealthyColor
		}
		dot, _ := walk.NewSolidColorBrush(dotColor)
		canvas.FillEllipse(dot, walk.Rectangle{X: row.X + 14, Y: row.Y + row.Height/2 - 5, Width: 10, Height: 10})
		dot.Dispose()
		text := walk.Rectangle{X: row.X + 36, Y: row.Y + 9, Width: row.Width - 46, Height: 22}
		canvas.DrawTextPixels(item.tunnel.Name, rail.boldFont, uiTextColor, text, walk.TextLeft|walk.TextSingleLine|walk.TextEndEllipsis)
		canvas.DrawTextPixels(textForState(item.state, false), rail.font, statusColor, walk.Rectangle{X: text.X, Y: row.Y + 31, Width: text.Width, Height: 18}, walk.TextLeft|walk.TextSingleLine|walk.TextEndEllipsis)
	}
	return nil
}
