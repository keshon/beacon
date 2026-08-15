package storage_test

import (
	"testing"
	"time"

	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/storage"
)

func TestDeleteMonitorNotFound(t *testing.T) {
	dir := t.TempDir()
	st, err := storage.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		st.Close()
	}()

	if err := st.DeleteMonitor("missing"); err != storage.ErrMonitorNotFound {
		t.Fatalf("expected ErrMonitorNotFound, got %v", err)
	}
}

func TestRecordCheckResultRejectsDeletedMonitor(t *testing.T) {
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
	if err := st.DeleteMonitor("m1"); err != nil {
		t.Fatal(err)
	}
	rec := storage.CheckRecord{MonitorID: "m1", Success: true, Time: time.Now()}
	stState := &monitor.MonitorState{MonitorID: "m1", Status: monitor.StatusUp}
	if err := st.RecordCheckResult(rec, stState, true); err != storage.ErrMonitorNotFound {
		t.Fatalf("expected ErrMonitorNotFound, got %v", err)
	}
}

// This asserts DURABILITY, not a file on disk: the old test read
// monitors.json and would have broken along with the engine even though
// the behaviour it cared about never changed.
func TestMonitorSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	st, err := storage.New(dir)
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
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	again, err := storage.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()

	got, err := again.GetMonitor("m1")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Name != "t" || got.Target != "example.com:80" {
		t.Fatalf("монитор не пережил перезапуск: %#v", got)
	}
}

// History is bounded PER MONITOR: a busy neighbour must not evict
// anyone else's samples, which is what the old global cap did.
func TestUptimeWindowIsPerMonitor(t *testing.T) {
	dir := t.TempDir()
	st, err := storage.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	for _, id := range []string{"a", "b"} {
		if err := st.SetMonitor(&monitor.Monitor{
			ID: id, Name: id, Type: "tcp", Target: "example.com:80", Enabled: true,
		}); err != nil {
			t.Fatal(err)
		}
	}
	base := time.Now().Add(-time.Hour)
	for i := 0; i < 600; i++ {
		if err := st.AppendCheckRecord(storage.CheckRecord{
			MonitorID: "a", Success: true, Time: base.Add(time.Duration(i) * time.Second),
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.AppendCheckRecord(storage.CheckRecord{
		MonitorID: "b", Success: true, Time: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	a, err := st.GetUptimeSamples("a", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 500 {
		t.Fatalf("окно монитора a: ждали 500, получили %d", len(a))
	}
	b, err := st.GetUptimeSamples("b", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 1 {
		t.Fatalf("сосед вытеснил записи монитора b: осталось %d", len(b))
	}
}

func TestUpdateMonitorAtomic(t *testing.T) {
	dir := t.TempDir()
	st, err := storage.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
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
