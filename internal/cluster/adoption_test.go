package cluster_test

import (
	"testing"
	"time"

	"github.com/keshon/beacon/internal/cluster"
	"github.com/keshon/beacon/internal/monitor"
)

func TestMergeMonitorStatePicksNewer(t *testing.T) {
	older := &monitor.MonitorState{
		MonitorID: "a",
		Status:    monitor.StatusUp,
		LastCheck: time.Now().Add(-2 * time.Minute),
	}
	newer := &monitor.MonitorState{
		MonitorID: "a",
		Status:    monitor.StatusDown,
		LastCheck: time.Now().Add(-1 * time.Minute),
	}
	got := cluster.MergeMonitorState(older, newer)
	if got.Status != monitor.StatusDown {
		t.Fatalf("expected down, got %s", got.Status)
	}
}

func TestMergeStateMaps(t *testing.T) {
	base := map[string]*monitor.MonitorState{
		"a": {MonitorID: "a", Status: monitor.StatusUp, LastCheck: time.Now()},
	}
	incoming := map[string]*monitor.MonitorState{
		"a": {MonitorID: "a", Status: monitor.StatusDown, LastCheck: time.Now().Add(-time.Hour)},
	}
	out := cluster.MergeStateMaps(base, incoming)
	if out["a"].Status != monitor.StatusUp {
		t.Fatalf("expected to keep newer base state")
	}
}
