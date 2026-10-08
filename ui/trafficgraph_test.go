package ui

import (
	"testing"
	"time"
)

func TestTrafficHistoryFirstSampleAndDelta(t *testing.T) {
	var history trafficHistory
	now := time.Unix(1, 0)
	history.sample(now, 100, 200)
	if got := history.latest(); got.rxBps != 0 || got.txBps != 0 {
		t.Fatalf("first sample = %#v", got)
	}
	history.sample(now.Add(2*time.Second), 300, 500)
	got := history.latest()
	if got.rxBps != 800 || got.txBps != 1200 {
		t.Fatalf("rates = %#v", got)
	}
}

func TestTrafficHistoryCounterResetAndLimit(t *testing.T) {
	var history trafficHistory
	now := time.Unix(1, 0)
	history.sample(now, 100, 100)
	history.sample(now.Add(time.Second), 50, 50)
	if got := history.latest(); got.rxBps != 0 || got.txBps != 0 {
		t.Fatalf("reset produced a spike: %#v", got)
	}
	for i := 0; i < trafficHistoryLimit+20; i++ {
		history.sample(now.Add(time.Duration(i+2)*time.Second), uint64(100+i), uint64(100+i))
	}
	if len(history.samples) != trafficHistoryLimit {
		t.Fatalf("sample count = %d", len(history.samples))
	}
}
