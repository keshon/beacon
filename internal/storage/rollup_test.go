package storage_test

import (
	"testing"
	"time"

	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/storage"
)

func newStore(t *testing.T) *storage.Store {
	t.Helper()
	st, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func addMonitor(t *testing.T, st *storage.Store, id string) {
	t.Helper()
	if err := st.SetMonitor(&monitor.Monitor{
		ID: id, Name: id, Type: "tcp", Target: "example.com:80", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
}

// An hour of checks must fold into exactly one bucket carrying the counts and
// the worst latency, whatever the raw samples' fate afterwards.
func TestRollupAggregatesOneHour(t *testing.T) {
	st := newStore(t)
	addMonitor(t, st, "m")

	hour := time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		rec := storage.CheckRecord{
			MonitorID: "m",
			Success:   i%5 != 0, // два падения из десяти
			Time:      hour.Add(time.Duration(i) * time.Minute),
			Latency:   time.Duration(10+i*10) * time.Millisecond,
		}
		if err := st.AppendCheckRecord(rec); err != nil {
			t.Fatal(err)
		}
	}

	got, err := st.GetRollups("m", hour, hour.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want one bucket, got %d", len(got))
	}
	r := got[0]
	if r.Total != 10 || r.Failed != 2 || r.Ok() != 8 {
		t.Fatalf("counts: total=%d failed=%d ok=%d", r.Total, r.Failed, r.Ok())
	}
	if r.LatMaxMs != 100 {
		t.Fatalf("worst latency: want 100ms, got %dms", r.LatMaxMs)
	}
	if want := 55 * time.Millisecond; r.AvgLatency() != want {
		t.Fatalf("average latency: want %v, got %v", want, r.AvgLatency())
	}
}

// Checks spread over several hours must land in separate buckets, and the
// range query must return them oldest first.
func TestRollupSplitsByHourAndOrders(t *testing.T) {
	st := newStore(t)
	addMonitor(t, st, "m")

	base := time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC)
	for h := 0; h < 5; h++ {
		for i := 0; i < 3; i++ {
			if err := st.AppendCheckRecord(storage.CheckRecord{
				MonitorID: "m", Success: true,
				Time: base.Add(time.Duration(h)*time.Hour + time.Duration(i)*time.Minute),
			}); err != nil {
				t.Fatal(err)
			}
		}
	}

	got, err := st.GetRollups("m", base, base.Add(5*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 {
		t.Fatalf("want five buckets, got %d", len(got))
	}
	for i, r := range got {
		if r.Total != 3 {
			t.Fatalf("bucket %d: want 3 checks, got %d", i, r.Total)
		}
		if i > 0 && got[i-1].Hour >= r.Hour {
			t.Fatalf("buckets are not oldest-first at %d", i)
		}
	}
}

// The bucket must outlive the raw samples it was built from — that is the whole
// point of keeping it.
func TestRollupSurvivesRawEviction(t *testing.T) {
	st := newStore(t)
	addMonitor(t, st, "m")

	hour := time.Now().UTC().Truncate(time.Hour).Add(-2 * time.Hour)
	const n = 600 // больше окна сырых записей
	for i := 0; i < n; i++ {
		if err := st.AppendCheckRecord(storage.CheckRecord{
			MonitorID: "m", Success: true,
			Time:    hour.Add(time.Duration(i) * time.Second),
			Latency: time.Millisecond,
		}); err != nil {
			t.Fatal(err)
		}
	}

	raw, err := st.GetUptimeSamples("m", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 500 {
		t.Fatalf("raw window: want 500, got %d", len(raw))
	}

	got, err := st.GetRollups("m", hour, hour.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, r := range got {
		total += r.Total
	}
	if total != n {
		t.Fatalf("buckets lost history: want %d checks, got %d", n, total)
	}
}

// A raw sample older than the age cap goes even when the count bound is nowhere
// near — that is what protects a monitor checked once an hour.
func TestRawAgeCapEvictsSparseHistory(t *testing.T) {
	st := newStore(t)
	addMonitor(t, st, "m")

	old := time.Now().Add(-30 * 24 * time.Hour)
	if err := st.AppendCheckRecord(storage.CheckRecord{
		MonitorID: "m", Success: true, Time: old,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.AppendCheckRecord(storage.CheckRecord{
		MonitorID: "m", Success: true, Time: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	raw, err := st.GetUptimeSamples("m", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 1 {
		t.Fatalf("want only the fresh sample, got %d", len(raw))
	}
	if time.Since(raw[0].Time) > time.Minute {
		t.Fatal("the stale sample survived the age cap")
	}
}

// The batch read is what the list screen uses; it must not mix monitors up.
func TestRollupsBatchKeepsMonitorsApart(t *testing.T) {
	st := newStore(t)
	addMonitor(t, st, "a")
	addMonitor(t, st, "b")

	hour := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 4; i++ {
		if err := st.AppendCheckRecord(storage.CheckRecord{
			MonitorID: "a", Success: true, Time: hour.Add(time.Duration(i) * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.AppendCheckRecord(storage.CheckRecord{
		MonitorID: "b", Success: false, Time: hour.Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}

	got, err := st.GetRollupsBatch([]string{"a", "b"}, hour, hour.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(got["a"]) != 1 || got["a"][0].Total != 4 || got["a"][0].Failed != 0 {
		t.Fatalf("monitor a: %#v", got["a"])
	}
	if len(got["b"]) != 1 || got["b"][0].Total != 1 || got["b"][0].Failed != 1 {
		t.Fatalf("monitor b: %#v", got["b"])
	}
}
