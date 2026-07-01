package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/monitor/scheduler"
	"github.com/keshon/beacon/internal/storage"
)

func TestStartupDownMonitors(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	st, err := storage.New(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
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

	cfg := config.Default()
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

func TestReloadConfig(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	st, err := storage.New(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		st.Close()
	}()

	cfg := config.Default()
	cfg.DefaultInterval = 30
	eval := monitor.NewStatusEvaluator(nil, nil)
	sch := scheduler.New(st, scheduler.LocalSource{Store: st}, eval, 2, 30*time.Second, cfg, nil)
	cfg.DefaultInterval = 60
	cfg.Workers = 5
	sch.ReloadConfig(cfg)
	// smoke: no panic; behavior verified indirectly via integration
}
