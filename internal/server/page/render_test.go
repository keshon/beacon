package page

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

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
		"2 of 72 checks failed", "3 incidents this week", "beacon-tick--gap", "10000ms"} {
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
