package storage_test

import (
	"testing"
	"time"

	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/storage"
)

// record drives one check through the store the way the scheduler does:
// an outcome plus the state it produced.
func record(t *testing.T, st *storage.Store, id, status string, at time.Time, errText string) {
	t.Helper()
	rec := storage.CheckRecord{
		MonitorID: id,
		Success:   status == monitor.StatusUp,
		Time:      at,
		Error:     errText,
	}
	state := &monitor.MonitorState{MonitorID: id, Status: status, LastCheck: at}
	if err := st.RecordCheckResult(rec, state, true); err != nil {
		t.Fatal(err)
	}
}

// Going down opens exactly one incident however many failed checks follow, and
// coming back closes it.
func TestIncidentOpensOnceAndCloses(t *testing.T) {
	st := newStore(t)
	addMonitor(t, st, "m")

	base := time.Now().Add(-time.Hour)
	record(t, st, "m", monitor.StatusDown, base, "connection timeout")
	record(t, st, "m", monitor.StatusDown, base.Add(time.Minute), "connection timeout")
	record(t, st, "m", monitor.StatusDown, base.Add(2*time.Minute), "connection timeout")

	open, err := st.GetMonitorIncidents("m", base.Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 1 {
		t.Fatalf("want one incident, got %d", len(open))
	}
	if !open[0].Ongoing() {
		t.Fatal("incident closed while the monitor is still down")
	}
	if open[0].Checks != 3 {
		t.Fatalf("failed checks: want 3, got %d", open[0].Checks)
	}
	if open[0].Reason != "connection timeout" {
		t.Fatalf("reason: %q", open[0].Reason)
	}

	record(t, st, "m", monitor.StatusUp, base.Add(5*time.Minute), "")

	closed, err := st.GetMonitorIncidents("m", base.Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(closed) != 1 {
		t.Fatalf("recovery should close, not add: got %d", len(closed))
	}
	if closed[0].Ongoing() {
		t.Fatal("incident still open after recovery")
	}
	if got := closed[0].Duration(time.Now()); got != 5*time.Minute {
		t.Fatalf("duration: want 5m, got %v", got)
	}
}

// Falling down twice is two incidents — that is what makes "third time this
// week" answerable.
func TestIncidentsCountRepeats(t *testing.T) {
	st := newStore(t)
	addMonitor(t, st, "m")

	base := time.Now().Add(-72 * time.Hour)
	for i := 0; i < 3; i++ {
		at := base.Add(time.Duration(i) * 24 * time.Hour)
		record(t, st, "m", monitor.StatusDown, at, "boom")
		record(t, st, "m", monitor.StatusUp, at.Add(6*time.Minute), "")
	}

	n, err := st.CountIncidents("m", time.Now().Add(-7*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("incidents this week: want 3, got %d", n)
	}
	if n, _ := st.CountIncidents("m", time.Now().Add(-time.Hour)); n != 0 {
		t.Fatalf("incidents in the last hour: want 0, got %d", n)
	}
}

// An unknown status is not a recovery: it must leave the incident running.
func TestUnknownDoesNotCloseIncident(t *testing.T) {
	st := newStore(t)
	addMonitor(t, st, "m")

	at := time.Now().Add(-10 * time.Minute)
	record(t, st, "m", monitor.StatusDown, at, "boom")
	record(t, st, "m", monitor.StatusUnknown, at.Add(time.Minute), "")

	list, err := st.GetMonitorIncidents("m", at.Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || !list[0].Ongoing() {
		t.Fatalf("unknown status closed the incident: %#v", list)
	}
}

// The fleet-wide read is what the incidents screen uses: newest first, across
// monitors.
func TestIncidentsAcrossMonitorsNewestFirst(t *testing.T) {
	st := newStore(t)
	addMonitor(t, st, "a")
	addMonitor(t, st, "b")

	base := time.Now().Add(-3 * time.Hour)
	record(t, st, "a", monitor.StatusDown, base, "a down")
	record(t, st, "b", monitor.StatusDown, base.Add(time.Hour), "b down")

	list, err := st.GetIncidents(base.Add(-time.Hour), time.Now(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("want two incidents, got %d", len(list))
	}
	if list[0].MonitorID != "b" || list[1].MonitorID != "a" {
		t.Fatalf("order is not newest-first: %s then %s", list[0].MonitorID, list[1].MonitorID)
	}
}

// Deleting a monitor must take everything that belonged to it: samples,
// incidents and hourly buckets alike.
func TestDeleteMonitorLeavesNothingBehind(t *testing.T) {
	st := newStore(t)
	addMonitor(t, st, "m")

	at := time.Now().Add(-time.Hour)
	record(t, st, "m", monitor.StatusDown, at, "boom")
	record(t, st, "m", monitor.StatusUp, at.Add(time.Minute), "")

	if err := st.DeleteMonitor("m"); err != nil {
		t.Fatal(err)
	}

	if raw, _ := st.GetUptimeSamples("m", 100); len(raw) != 0 {
		t.Fatalf("samples left behind: %d", len(raw))
	}
	if inc, _ := st.GetMonitorIncidents("m", at.Add(-time.Hour), time.Now()); len(inc) != 0 {
		t.Fatalf("incidents left behind: %d", len(inc))
	}
	if roll, _ := st.GetRollups("m", at.Add(-time.Hour), time.Now()); len(roll) != 0 {
		t.Fatalf("hourly buckets left behind: %d", len(roll))
	}
}
