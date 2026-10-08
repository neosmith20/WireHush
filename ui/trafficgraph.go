//go:build windows

/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2026 WireHush contributors. All Rights Reserved.
 */

package ui

import (
	"fmt"
	"math"
	"time"

	"github.com/lxn/walk"
	"golang.zx2c4.com/wireguard/windows/conf"
)

const trafficHistoryLimit = 300

type trafficSample struct {
	at    time.Time
	rxBps float64
	txBps float64
}

type trafficHistory struct {
	samples []trafficSample
	prevAt  time.Time
	prevRx  uint64
	prevTx  uint64
	ready   bool
}

func peerCounters(peers []conf.Peer) (rx, tx uint64) {
	for _, peer := range peers {
		rx += uint64(peer.RxBytes)
		tx += uint64(peer.TxBytes)
	}
	return
}

func (history *trafficHistory) sample(at time.Time, rx, tx uint64) {
	if !history.ready {
		history.ready, history.prevAt, history.prevRx, history.prevTx = true, at, rx, tx
		history.samples = append(history.samples, trafficSample{at: at})
		return
	}
	elapsed := at.Sub(history.prevAt).Seconds()
	if elapsed <= 0 {
		return
	}
	sample := trafficSample{at: at}
	if rx >= history.prevRx {
		sample.rxBps = float64(rx-history.prevRx) * 8 / elapsed
	}
	if tx >= history.prevTx {
		sample.txBps = float64(tx-history.prevTx) * 8 / elapsed
	}
	history.samples = append(history.samples, sample)
	if len(history.samples) > trafficHistoryLimit {
		history.samples = history.samples[len(history.samples)-trafficHistoryLimit:]
	}
	history.prevAt, history.prevRx, history.prevTx = at, rx, tx
}

func (history *trafficHistory) reset() { *history = trafficHistory{} }

func (history *trafficHistory) latest() trafficSample {
	if len(history.samples) == 0 {
		return trafficSample{}
	}
	return history.samples[len(history.samples)-1]
}

func formatRate(bits float64) string {
	units := []string{"bps", "Kbps", "Mbps", "Gbps"}
	value := bits
	unit := 0
	for value >= 1000 && unit < len(units)-1 {
		value /= 1000
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%.0f %s", value, units[unit])
	}
	return fmt.Sprintf("%.1f %s", value, units[unit])
}

func formatBytes(bytes uint64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	value := float64(bytes)
	unit := 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%.0f %s", value, units[unit])
	}
	return fmt.Sprintf("%.1f %s", value, units[unit])
}

type trafficGraph struct {
	*walk.CustomWidget
	history *trafficHistory
	font    *walk.Font
}

func newTrafficGraph(parent walk.Container, history *trafficHistory) (*trafficGraph, error) {
	graph := &trafficGraph{history: history}
	// The constructor is allowed to paint immediately, before the widget has
	// been assigned, so initialise the text resource first.
	graph.font, _ = walk.NewFont("Segoe UI", 8, 0)
	widget, err := walk.NewCustomWidgetPixels(parent, 0, graph.paint)
	if err != nil {
		return nil, err
	}
	graph.CustomWidget = widget
	graph.SetPaintMode(walk.PaintBuffered)
	graph.SetInvalidatesOnResize(true)
	graph.SetMinMaxSize(walk.Size{0, 164}, walk.Size{0, 164})
	return graph, nil
}

func (graph *trafficGraph) paint(canvas *walk.Canvas, bounds walk.Rectangle) error {
	canvas.FillRectangle(uiCardBrush, bounds)
	if len(graph.history.samples) < 2 {
		return canvas.DrawTextPixels("Waiting for traffic…", graph.font, uiMutedColor, bounds, walk.TextCenter|walk.TextVCenter|walk.TextSingleLine)
	}
	maxRate := 1.0
	for _, sample := range graph.history.samples {
		maxRate = math.Max(maxRate, math.Max(sample.rxBps, sample.txBps))
	}
	maxRate = niceRate(maxRate)
	gridPen, _ := walk.NewCosmeticPen(walk.PenSolid, walk.RGB(36, 56, 70))
	defer gridPen.Dispose()
	for i := 0; i < 4; i++ {
		y := bounds.Y + bounds.Height - 1 - i*(bounds.Height-1)/3
		canvas.DrawLinePixels(gridPen, walk.Point{X: bounds.X, Y: y}, walk.Point{X: bounds.X + bounds.Width, Y: y})
	}
	rxPen, _ := walk.NewCosmeticPen(walk.PenSolid, uiAccentColor)
	txPen, _ := walk.NewCosmeticPen(walk.PenSolid, uiHealthyColor)
	defer rxPen.Dispose()
	defer txPen.Dispose()
	for i := 1; i < len(graph.history.samples); i++ {
		from, to := graph.history.samples[i-1], graph.history.samples[i]
		x1 := bounds.X + (i-1)*(bounds.Width-1)/(len(graph.history.samples)-1)
		x2 := bounds.X + i*(bounds.Width-1)/(len(graph.history.samples)-1)
		y := func(rate float64) int {
			return bounds.Y + bounds.Height - 1 - int(rate/maxRate*float64(bounds.Height-1))
		}
		canvas.DrawLinePixels(rxPen, walk.Point{X: x1, Y: y(from.rxBps)}, walk.Point{X: x2, Y: y(to.rxBps)})
		canvas.DrawLinePixels(txPen, walk.Point{X: x1, Y: y(from.txBps)}, walk.Point{X: x2, Y: y(to.txBps)})
	}
	return nil
}

func niceRate(value float64) float64 {
	base := math.Pow(10, math.Floor(math.Log10(value)))
	for _, multiplier := range []float64{1, 2, 5, 10} {
		if value <= multiplier*base {
			return multiplier * base
		}
	}
	return 10 * base
}
