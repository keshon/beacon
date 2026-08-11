package page

import (
	"strings"
	"testing"
	"time"

	"github.com/keshon/beacon/internal/monitor"
)

func stateWithCert(id string, at time.Time) *monitor.MonitorState {
	return &monitor.MonitorState{MonitorID: id, Status: monitor.StatusUp, CertExpiry: at}
}

// The note is a warning, and a warning that is always on screen is furniture.
func TestCertNoteStaysQuietWhileThereIsTime(t *testing.T) {
	now := time.Now()
	states := map[string]*monitor.MonitorState{
		"m1": stateWithCert("m1", now.Add(90*24*time.Hour)),
	}
	if got := buildCertNote(states, map[string]string{"m1": "shop"}, now); got != nil {
		t.Fatalf("a certificate three months out should say nothing, got %q", got.Text)
	}
}

func TestCertNoteSpeaksAboutTheNearest(t *testing.T) {
	now := time.Now()
	states := map[string]*monitor.MonitorState{
		"m1": stateWithCert("m1", now.Add(60*24*time.Hour)),
		"m2": stateWithCert("m2", now.Add(9*24*time.Hour)),
		"m3": stateWithCert("m3", now.Add(40*24*time.Hour)),
	}
	names := map[string]string{"m1": "shop", "m2": "api", "m3": "cdn"}

	got := buildCertNote(states, names, now)
	if got == nil {
		t.Fatal("nine days out and the note is silent")
	}
	if got.MonitorID != "m2" {
		t.Fatalf("the note points at %q, not at the nearest deadline", got.MonitorID)
	}
	for _, want := range []string{"api", "9 days", "2 more tracked"} {
		if !strings.Contains(got.Text, want) {
			t.Fatalf("note %q is missing %q", got.Text, want)
		}
	}
	if got.Tone != "warn" {
		t.Fatalf("tone: want warn, got %q", got.Tone)
	}
}

func TestCertNoteWordsTheEdges(t *testing.T) {
	now := time.Now()
	names := map[string]string{"m1": "shop"}

	expired := buildCertNote(map[string]*monitor.MonitorState{
		"m1": stateWithCert("m1", now.Add(-time.Hour)),
	}, names, now)
	if expired == nil || !strings.Contains(expired.Text, "has expired") {
		t.Fatalf("an expired certificate is not worded as expired: %#v", expired)
	}

	soon := buildCertNote(map[string]*monitor.MonitorState{
		"m1": stateWithCert("m1", now.Add(3*time.Hour)),
	}, names, now)
	if soon == nil || !strings.Contains(soon.Text, "within a day") {
		t.Fatalf("hours left should not be rounded to days: %#v", soon)
	}

	one := buildCertNote(map[string]*monitor.MonitorState{
		"m1": stateWithCert("m1", now.Add(25*time.Hour)),
	}, names, now)
	if one == nil || !strings.Contains(one.Text, "1 day") || strings.Contains(one.Text, "1 days") {
		t.Fatalf("singular day is mis-worded: %#v", one)
	}
}

// A monitor with no TLS, or one that has never completed a handshake, must not
// produce a line at all — silence is the correct answer, not "unknown".
func TestCertNoteIgnoresMonitorsWithoutTLS(t *testing.T) {
	now := time.Now()
	states := map[string]*monitor.MonitorState{
		"m1": {MonitorID: "m1", Status: monitor.StatusUp},
	}
	if got := buildCertNote(states, map[string]string{"m1": "db"}, now); got != nil {
		t.Fatalf("a monitor without a certificate produced %q", got.Text)
	}
}

// Peers have state but no local monitor to link to; skipping them keeps the
// note from pointing at a screen that does not exist.
func TestCertNoteSkipsUnnamedMonitors(t *testing.T) {
	now := time.Now()
	states := map[string]*monitor.MonitorState{
		"peer-1": stateWithCert("peer-1", now.Add(2*24*time.Hour)),
	}
	if got := buildCertNote(states, map[string]string{}, now); got != nil {
		t.Fatalf("a peer produced a note: %q", got.Text)
	}
}

func TestCertLineOnMonitorScreen(t *testing.T) {
	now := time.Now()

	text, tone := certLine(stateWithCert("m1", now.Add(200*24*time.Hour)), now)
	if tone != "" || !strings.HasPrefix(text, "until ") {
		t.Fatalf("a distant date should be plain, got %q / %q", text, tone)
	}

	text, tone = certLine(stateWithCert("m1", now.Add(5*24*time.Hour)), now)
	if tone != "warn" || !strings.Contains(text, "left, until ") {
		t.Fatalf("a near date should warn and say how long, got %q / %q", text, tone)
	}

	text, tone = certLine(stateWithCert("m1", now.Add(-24*time.Hour)), now)
	if tone != "error" || !strings.HasPrefix(text, "expired ") {
		t.Fatalf("a passed date should read as expired, got %q / %q", text, tone)
	}

	if text, _ := certLine(nil, now); text != "" {
		t.Fatalf("no state should give no line, got %q", text)
	}
}
