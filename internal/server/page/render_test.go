package page

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/keshon/beacon/internal/cluster"
	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/server/httpx"

	"github.com/flosch/pongo2/v6"
)

// Templates have branches a running server does not necessarily walk: the
// incidents table only renders once something has broken, and an empty store
// shows the other side. A broken branch would sit there until the day it is
// needed most, so both are rendered here.

func tplDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "..", "templates"))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func render(t *testing.T, name string, ctx pongo2.Context) string {
	t.Helper()
	rec := httptest.NewRecorder()
	if err := httpx.Render(rec, tplDir(t), name, ctx); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	body := rec.Body.String()
	if strings.Contains(body, "[Error") {
		t.Fatalf("%s rendered an error: %s", name, body[:min(300, len(body))])
	}
	return body
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestIncidentsScreenRendersBothBranches(t *testing.T) {
	full := render(t, "dashboard/incidents.html", pongo2.Context{
		"version": "test", "nav_active": "incidents", "window": "week",
		"total": 3, "ongoing": 1, "downtime": "22m",
		"incidents": []fleetIncident{
			{MonitorID: "m1", Name: "shop.example.com", Started: "11 Aug, 14:35",
				Duration: "running 4m", Reason: "connection timeout", Ongoing: true, Repeats: 3},
			{MonitorID: "m2", Name: "api.example.com", Started: "11 Aug, 14:33",
				Duration: "4m", Reason: "502 from gateway", Repeats: 1},
		},
	})
	for _, want := range []string{"shop.example.com", "connection timeout", "3×", "1 running now", "22m"} {
		if !strings.Contains(full, want) {
			t.Fatalf("populated incidents screen is missing %q", want)
		}
	}

	empty := render(t, "dashboard/incidents.html", pongo2.Context{
		"version": "test", "nav_active": "incidents", "window": "day",
		"total": 0, "ongoing": 0, "downtime": "0s", "incidents": []fleetIncident{},
	})
	if !strings.Contains(empty, "Nothing broke in this period") {
		t.Fatal("empty incidents screen lost its empty state")
	}
	if strings.Contains(empty, "inst-table") {
		t.Fatal("empty incidents screen still renders the table")
	}
}

func TestMonitorScreenRendersBothBranches(t *testing.T) {
	mon := &monitor.Monitor{
		ID: "m1", Name: "shop.example.com", Type: "http",
		Target: "https://shop.example.com/health", Enabled: true, Retries: 2,
	}
	base := pongo2.Context{
		"version": "test", "nav_active": "dashboard", "mon": mon,
		"status": monitor.StatusDown, "enabled": true, "intervalSec": 60,
		"history": []HistTick{
			{Title: "10:00 — 120 checks"},
			{Tone: "error", Title: "11:00 — 4 of 120 failed"},
			{Gap: true, Title: "12:00 — no checks"},
		},
		"historyLabel": "Last 24 hours: 1 hour with failures",
		"windows": []uptimeWindow{
			{Label: "24 hours", Percent: "97.22%", Tone: "warn", Downtime: "2 of 72 checks failed", Known: true},
			{Label: "30 days", Percent: "—", Downtime: "no checks"},
		},
		"repeats": 3,
	}

	base["steps"] = []stepRow{
		{Name: "DNS", Sub: "name resolved", Meta: "12ms", State: "ok"},
		{Name: "TLS", Sub: "certificate 5d left, until 16 August 2026", Meta: "210ms", State: "warn"},
		{Name: "HTTP", Sub: "no response", Meta: "10.0s · timeout", State: "failed"},
	}
	base["lastCheck"] = "14:35:07"

	full := base
	full["incidents"] = []incidentRow{
		{Started: "11 Aug, 14:35", Duration: "running 4m", Reason: "connection timeout", Ongoing: true, Tone: "error"},
	}
	full["checks"] = []checkRow{
		{Time: "14:35:07", Latency: "10000ms", OK: false, Error: "timeout"},
		{Time: "14:34:07", Latency: "38ms", OK: true},
	}
	body := render(t, "dashboard/monitor/detail.html", full)
	for _, want := range []string{"shop.example.com", "connection timeout", "97.22%",
		"2 of 72 checks failed", "3 incidents this week", "beacon-tick--gap", "10000ms",
		"inst-step", "name resolved", "10.0s · timeout", "14:35:07"} {
		if !strings.Contains(body, want) {
			t.Fatalf("monitor screen is missing %q", want)
		}
	}

	bare := base
	bare["incidents"] = []incidentRow{}
	bare["checks"] = []checkRow{}
	empty := render(t, "dashboard/monitor/detail.html", bare)
	if !strings.Contains(empty, "No incidents recorded") || !strings.Contains(empty, "No checks yet") {
		t.Fatal("a fresh monitor lost one of its empty states")
	}
}

// The outage block is only ever on screen when something is broken, which is
// exactly when a broken template would be least welcome.
func TestDashboardOutageBlock(t *testing.T) {
	ctx := pongo2.Context{
		"version": "test", "nav_active": "dashboard",
		"window": "day", "windowLabel": "24 hours",
		"windows":    histWindows,
		"fleetTone":  "error",
		"fleetLabel": "2 not responding",
		"countTotal": 2, "countUp": 0, "countDown": 2, "countPaused": 0,
		"rows": []dashboardRow{}, "hasNetwork": false, "networkEnabled": false,
		"outage": &outageBlock{
			Head:   "shop.example.com and api.example.com are not responding",
			Reason: "connection timeout",
			Tried: []string{
				"4 checks in a row failed since 14:31",
				"3 outages of shop.example.com this week — it repeats",
			},
			First: "m1",
		},
	}
	body := render(t, "dashboard/dashboard.html", ctx)
	for _, want := range []string{
		"inst-failure", "are not responding", "connection timeout",
		"4 checks in a row failed", "it repeats", "/monitors/m1",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("outage block is missing %q", want)
		}
	}
	// The block exists so that something can be DONE about the outage from
	// where it is read. It used to offer two links to other screens, which is
	// the friction it was supposed to remove; the assertion is here so that a
	// tidy-up does not quietly turn the actions back into navigation.
	for _, want := range []string{
		`data-action="check" data-monitor="m1"`,
		`data-action="mute" data-minutes="60" data-monitor="m1"`,
		`data-action="ack" data-monitor="m1"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("outage block is missing the action %q", want)
		}
	}

	ctx["outage"] = nil
	ctx["fleetTone"] = "ok"
	ctx["fleetLabel"] = "all 2 responding"
	quiet := render(t, "dashboard/dashboard.html", ctx)
	if strings.Contains(quiet, "inst-failure") {
		t.Fatal("the outage block shows up when nothing is down")
	}
}

// Two horizons meet on the summary and neither branch is guaranteed to be on
// screen when someone opens it, so both are rendered here.
func TestSummaryScreenRendersBothBranches(t *testing.T) {
	base := pongo2.Context{
		"version": "test", "nav_active": "summary",
		"window": "month", "windowLabel": "30 days",
		"windows":   summaryWindows,
		"clean":     13,
		"tracked":   19,
		"incidents": 31,
		"median":    "6m",
		"certs": []certRow{
			{ID: "m1", Name: "shop", Left: "9d", Until: "20 Aug 2026", Tone: "warn"},
			{ID: "m2", Name: "api", Left: "34d", Until: "14 Sep 2026"},
		},
	}

	full := base
	full["worst"] = []worstRow{
		{ID: "m1", Name: "shop", Failed: 9, Total: 1200, Percent: "99.25%", Incidents: 3,
			History: []HistTick{{Tone: "error", Title: "x"}, {Gap: true, Title: "y"}}},
	}
	body := render(t, "dashboard/summary.html", full)
	for _, want := range []string{
		"Where the failures were", "shop", "9 of 1200", "99.25%",
		"13", "of 19", "Median incident", "6m", "Certificates", "20 Aug 2026",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("summary is missing %q", want)
		}
	}

	quiet := base
	quiet["worst"] = []worstRow{}
	quiet["certs"] = []certRow{}
	empty := render(t, "dashboard/summary.html", quiet)
	if !strings.Contains(empty, "Nothing failed in this period") {
		t.Fatal("a clean period lost its empty state")
	}
	if strings.Contains(empty, "Certificates") {
		t.Fatal("the certificates panel shows with nothing to show")
	}
}

// The peers screen has three faces and only one of them is on display at a
// time: sync off, sync on with nothing configured, and a peer that is failing.
func TestPeersScreenRendersEveryState(t *testing.T) {
	off := render(t, "dashboard/peers.html", pongo2.Context{
		"version": "test", "nav_active": "peers",
		"enabled": false, "peers": []peerRow{}, "live": 0, "dead": 0, "warnings": 0,
	})
	if !strings.Contains(off, "Peer sync is off") {
		t.Fatal("a single-node install lost its explanation")
	}

	none := render(t, "dashboard/peers.html", pongo2.Context{
		"version": "test", "nav_active": "peers",
		"enabled": true, "peers": []peerRow{}, "live": 0, "dead": 0, "warnings": 0,
	})
	if !strings.Contains(none, "No peers configured") {
		t.Fatal("sync on with no addresses lost its state")
	}

	full := render(t, "dashboard/peers.html", pongo2.Context{
		"version": "test", "nav_active": "peers", "enabled": true,
		"live": 1, "dead": 1, "warnings": 2,
		"peers": []peerRow{
			{NetworkNode: cluster.NetworkNode{
				NodeID: "aaaa", NodeIDShort: "aaaa", URL: "https://here",
				Status: "self", MonitorsCount: 16, LastSeen: "just now",
			}, Self: true, Tone: "running"},
			{NetworkNode: cluster.NetworkNode{
				NodeID: "bbbb", NodeIDShort: "bbbb", URL: "https://there",
				Status: "dead", MonitorsCount: 1, LastSeen: "2h ago",
				LastError:    "HTTP 401 — check sync_token matches on both nodes",
				SyncWarnings: []string{"clock drift 4s", "schema is older"},
			}, Tone: "error"},
		},
	})
	for _, want := range []string{
		"this node", "16 monitor", "https://there",
		"Sync with https://there is failing", "check sync_token",
		"clock drift 4s", "2 sync warnings", "1 live, 1 unreachable",
	} {
		if !strings.Contains(full, want) {
			t.Fatalf("peers screen is missing %q", want)
		}
	}
}

// The delivery trail is read after a bad night, and the suppression rows are
// the reason it exists. Both faces of the screen are rendered here.
func TestNotificationsScreenRendersBothBranches(t *testing.T) {
	blank := render(t, "dashboard/notifications.html", pongo2.Context{
		"version": "test", "nav_active": "notifications",
		"anyConfig": false, "channels": []channelCard{}, "trail": []deliveryRow{},
	})
	if !strings.Contains(blank, "No channel is set up") {
		t.Fatal("an install with no channels lost its explanation")
	}
	if !strings.Contains(blank, "nobody is being told") {
		t.Fatal("the blank state stopped saying what it costs")
	}

	full := render(t, "dashboard/notifications.html", pongo2.Context{
		"version": "test", "nav_active": "notifications", "anyConfig": true,
		"channels": []channelCard{
			{Name: "telegram", Configured: true, Targets: 2, Sent: 14, Skipped: 3,
				LastAt: "11 Aug, 14:35", Tone: "ok", Verdict: "delivering"},
			{Name: "email", Configured: true, Targets: 1, Failed: 4,
				LastAt: "11 Aug, 14:35", LastError: "550 mailbox unavailable",
				Tone: "error", Verdict: "every attempt failed"},
			{Name: "discord", Verdict: "not configured"},
			{Name: "webhook", Configured: true, Targets: 1, Verdict: "configured, never used"},
		},
		"trail": []deliveryRow{
			{At: "11 Aug, 14:35:07", Monitor: "shop", Channel: "email", Label: "ops@example.com",
				Kind: "down", Status: "failed", Reason: "550 mailbox unavailable", Tone: "error"},
			{At: "11 Aug, 14:35:06", Monitor: "shop", Channel: "telegram", Label: "chat 42",
				Kind: "down", Status: "skipped", Reason: "policy alerts once and it already did", Tone: "warn"},
			{At: "11 Aug, 14:31:02", Monitor: "api", Channel: "telegram", Label: "chat 42",
				Kind: "recovered", Status: "sent"},
		},
	})
	for _, want := range []string{
		"Delivery trail", "550 mailbox unavailable", "policy alerts once and it already did",
		"ops@example.com", "chat 42", "configured, never used", "Suppressed",
	} {
		if !strings.Contains(full, want) {
			t.Fatalf("notifications screen is missing %q", want)
		}
	}
}
