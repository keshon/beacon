package page

import (
	"testing"
	"time"

	"github.com/keshon/beacon/internal/monitor"
)

func stateWithPhases(status string, p monitor.CheckPhases) *monitor.MonitorState {
	return &monitor.MonitorState{MonitorID: "m", Status: status, Phases: p, Latency: 10 * time.Second}
}

func names(rows []stepRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Name
	}
	return out
}

// A full HTTPS check walks the whole chain, and every step demonstrably
// succeeded because the next one happened.
func TestStepsFullChain(t *testing.T) {
	rows := buildSteps(stateWithPhases(monitor.StatusUp, monitor.CheckPhases{
		DNS: 12 * time.Millisecond, TCP: 38 * time.Millisecond,
		TLS: 210 * time.Millisecond, Server: 140 * time.Millisecond,
	}), "https://shop.example.com/", time.Now())

	want := []string{"DNS", "TCP", "TLS", "HTTP"}
	got := names(rows)
	if len(got) != len(want) {
		t.Fatalf("steps: want %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("step %d: want %s, got %s", i, want[i], got[i])
		}
		if rows[i].State != "ok" {
			t.Fatalf("%s should be ok on a passing check, got %s", got[i], rows[i].State)
		}
	}
	if rows[0].Meta != "12ms" || rows[2].Meta != "210ms" {
		t.Fatalf("timings are mis-formatted: %q %q", rows[0].Meta, rows[2].Meta)
	}
}

// A reused connection measures no DNS, TCP or TLS. Those steps must be absent,
// not shown as instant: zero there is a fact about the connection.
func TestStepsSkipWhatWasNotMeasured(t *testing.T) {
	rows := buildSteps(stateWithPhases(monitor.StatusUp, monitor.CheckPhases{
		Server: 40 * time.Millisecond,
	}), "https://shop.example.com/", time.Now())

	if got := names(rows); len(got) != 1 || got[0] != "HTTP" {
		t.Fatalf("want only HTTP, got %v", got)
	}
}

// Plain HTTP has no handshake, so no TLS row — and the check still passes.
func TestStepsPlainHTTPHasNoTLS(t *testing.T) {
	rows := buildSteps(stateWithPhases(monitor.StatusUp, monitor.CheckPhases{
		DNS: time.Millisecond, TCP: 2 * time.Millisecond, Server: 30 * time.Millisecond,
	}), "http://example.com/", time.Now())

	for _, r := range rows {
		if r.Name == "TLS" {
			t.Fatal("plain HTTP produced a TLS step")
		}
	}
}

// The useful half of a timeout: everything up to the server worked, and the
// last step is the one that failed.
func TestStepsTimeoutBlamesTheLastLink(t *testing.T) {
	rows := buildSteps(stateWithPhases(monitor.StatusDown, monitor.CheckPhases{
		DNS: 12 * time.Millisecond, TCP: 38 * time.Millisecond, TLS: 210 * time.Millisecond,
	}), "https://shop.example.com/", time.Now())

	if got := names(rows); len(got) != 4 || got[3] != "HTTP" {
		t.Fatalf("want the chain to end in HTTP, got %v", got)
	}
	for _, r := range rows[:3] {
		if r.State != "ok" {
			t.Fatalf("%s should stay ok: it demonstrably worked", r.Name)
		}
	}
	last := rows[3]
	if last.State != "failed" || last.Sub != "no response" {
		t.Fatalf("last step: %#v", last)
	}
	if last.Meta != "10.0s · timeout" {
		t.Fatalf("timeout meta: %q", last.Meta)
	}
}

// An expiring certificate belongs to the handshake that saw it.
func TestStepsAttachCertWarningToTLS(t *testing.T) {
	now := time.Now()
	st := stateWithPhases(monitor.StatusUp, monitor.CheckPhases{
		DNS: time.Millisecond, TCP: time.Millisecond,
		TLS: 200 * time.Millisecond, Server: 40 * time.Millisecond,
	})
	st.CertExpiry = now.Add(5 * 24 * time.Hour)

	rows := buildSteps(st, "https://shop.example.com/", now)
	var tls *stepRow
	for i := range rows {
		if rows[i].Name == "TLS" {
			tls = &rows[i]
		}
	}
	if tls == nil {
		t.Fatal("no TLS step")
	}
	if tls.State != "warn" {
		t.Fatalf("TLS should warn about an expiring certificate, got %s", tls.State)
	}
	if got := tls.Sub; got == "handshake finished" {
		t.Fatal("the certificate warning did not reach the TLS step")
	}
}

// Nothing measured means nothing to show — not a panel of dashes.
func TestStepsEmptyWhenNothingMeasured(t *testing.T) {
	if rows := buildSteps(nil, "", time.Now()); rows != nil {
		t.Fatalf("no state produced %d steps", len(rows))
	}
	if rows := buildSteps(stateWithPhases(monitor.StatusUp, monitor.CheckPhases{}), "", time.Now()); rows != nil {
		t.Fatalf("no phases produced %d steps", len(rows))
	}
}
