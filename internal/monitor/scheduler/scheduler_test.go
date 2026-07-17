package scheduler_test

import (
	"testing"
	"time"

	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/monitor/scheduler"
	"github.com/keshon/beacon/internal/storage"
)

func TestStartupDownMonitors(t *testing.T) {
	dir := t.TempDir()
	st, err := storage.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		st.Close()
	}()

	m := &monitor.Monitor{
		ID: "m1", Name: "t", Type: "tcp", Target: "example.com:80",
		Enabled: true, Retries: 1, Timeout: time.Second,
	}
	if err := st.SetMonitor(m); err != nil {
		t.Fatal(err)
	}
	if err := st.SetState(&monitor.MonitorState{
		MonitorID: "m1",
		Status:    monitor.StatusDown,
		LastCheck: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	cfg := config.NewLive(config.Default())
	eval := monitor.NewStatusEvaluator(nil, nil)
	sch := scheduler.New(st, scheduler.LocalSource{Store: st}, eval, 1, time.Second, cfg, nil)
	down, states, err := sch.StartupDownMonitors()
	if err != nil {
		t.Fatal(err)
	}
	if len(down) != 1 || states["m1"].Status != monitor.StatusDown {
		t.Fatalf("unexpected startup down set: %#v %#v", down, states)
	}
}

func TestLiveConfigSwap(t *testing.T) {
	dir := t.TempDir()
	st, err := storage.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		st.Close()
	}()

	base := config.Default()
	base.DefaultInterval = 30
	live := config.NewLive(base)
	eval := monitor.NewStatusEvaluator(nil, nil)
	_ = scheduler.New(st, scheduler.LocalSource{Store: st}, eval, 2, 30*time.Second, live, nil)
	next := *base
	next.DefaultInterval = 60
	next.Workers = 5
	live.Store(&next)
	if live.Load().DefaultInterval != 60 {
		t.Fatalf("expected swapped config, got %d", live.Load().DefaultInterval)
	}
}
