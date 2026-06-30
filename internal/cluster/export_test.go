package cluster_test

import (
	"context"
	"testing"
	"time"

	"github.com/keshon/beacon/internal/cluster"
	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/store"
)

func TestExportView_adoptedMonitorUsesLocalState(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, err := store.New(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		st.Close()
	}()

	monID := "adopted-1"
	deadOwner := "node-a"
	now := time.Now()

	cfg := &config.Config{
		Network: config.NetworkConfig{
			Enabled:     true,
			NodeID:      "node-b",
			DeadTimeout: 300,
		},
	}

	adoptedMon := &monitor.Monitor{
		ID:      monID,
		Name:    "keshon.ru",
		Type:    "http",
		Target:  "https://keshon.ru",
		Enabled: true,
	}

	staleUp := now.Add(-10 * time.Minute)
	freshDown := now.Add(-1 * time.Minute)

	peerData := map[string]*store.PeerData{
		deadOwner: {
			NodeID:   deadOwner,
			PeerURL:  "https://node-a.example.com",
			LastSeen: now.Add(-1 * time.Hour),
			Monitors: []*monitor.Monitor{adoptedMon},
			State: map[string]*monitor.MonitorState{
				monID: {
					MonitorID: monID,
					Status:    monitor.StatusUp,
					LastCheck: staleUp,
				},
			},
		},
	}
	for _, pd := range peerData {
		if err := st.SetPeerData(pd); err != nil {
			t.Fatal(err)
		}
	}

	if err := st.SetState(&monitor.MonitorState{
		MonitorID: monID,
		Status:    monitor.StatusDown,
		LastCheck: freshDown,
	}); err != nil {
		t.Fatal(err)
	}

	rt := cluster.New(st, cfg)
	view, err := rt.ExportView()
	if err != nil {
		t.Fatal(err)
	}

	stOut := view.State[monID]
	if stOut == nil {
		t.Fatal("expected exported state for adopted monitor")
	}
	if stOut.Status != monitor.StatusDown {
		t.Fatalf("expected DOWN from adopter local state, got %s", stOut.Status)
	}
	if !stOut.LastCheck.Equal(freshDown) {
		t.Fatalf("expected fresh LastCheck, got %v", stOut.LastCheck)
	}
}
