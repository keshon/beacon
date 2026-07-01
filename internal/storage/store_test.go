package storage_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/storage"
)

func TestDeleteMonitorNotFound(t *testing.T) {
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

	if err := st.DeleteMonitor("missing"); err != storage.ErrMonitorNotFound {
		t.Fatalf("expected ErrMonitorNotFound, got %v", err)
	}
}

func TestRecordCheckResultRejectsDeletedMonitor(t *testing.T) {
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
	if err := st.DeleteMonitor("m1"); err != nil {
		t.Fatal(err)
	}
	rec := storage.CheckRecord{MonitorID: "m1", Success: true, Time: time.Now()}
	stState := &monitor.MonitorState{MonitorID: "m1", Status: monitor.StatusUp}
	if err := st.RecordCheckResult(rec, stState, true); err != storage.ErrMonitorNotFound {
		t.Fatalf("expected ErrMonitorNotFound, got %v", err)
	}
}

func TestFlushPersistsMonitor(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	st, err := storage.New(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}

	m := &monitor.Monitor{
		ID: "m1", Name: "t", Type: "tcp", Target: "example.com:80",
		Enabled: true, Retries: 1, Timeout: time.Second,
	}
	if err := st.SetMonitor(m); err != nil {
		t.Fatal(err)
	}
	cancel()
	st.Close()

	raw, err := os.ReadFile(filepath.Join(dir, "monitors.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 10 {
		t.Fatalf("expected flushed monitors file, got %q", raw)
	}
}

func TestUpdateMonitorAtomic(t *testing.T) {
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
		ID: "m1", Name: "before", Type: "tcp", Target: "example.com:80",
		Enabled: true, Retries: 1, Timeout: time.Second,
	}
	if err := st.SetMonitor(m); err != nil {
		t.Fatal(err)
	}
	_, err = st.UpdateMonitor("m1", func(mon *monitor.Monitor) error {
		mon.Name = "after"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := st.GetMonitor("m1")
	if err != nil || got.Name != "after" {
		t.Fatalf("update failed: %v %#v", err, got)
	}
}
