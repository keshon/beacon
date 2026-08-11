package page

import (
	"net/http"
	"time"

	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/server/httpx"
	"github.com/keshon/beacon/internal/storage"

	"github.com/flosch/pongo2/v6"
)

// The incidents screen.
//
// It answers a question no other screen can: WHAT BREAKS REPEATEDLY. The
// dashboard shows now, the monitor screen shows one monitor's own story. Only
// here do three outages of the same host in one week line up next to each
// other, and three in a column is a cause rather than three events.
type Incidents struct {
	Store  *storage.Store
	Cfg    *config.Live
	TplDir string
}

type fleetIncident struct {
	MonitorID string
	Name      string
	Started   string
	Duration  string
	Reason    string
	Ongoing   bool
	// Repeats is how many times this monitor fell inside the window. Shown on
	// every row of the same monitor on purpose: three identical marks down a
	// column ARE the pattern, made visible.
	Repeats int
}

// incidentWindows are the periods the screen offers, longest label first match.
var incidentWindows = map[string]time.Duration{
	"day":   24 * time.Hour,
	"week":  7 * 24 * time.Hour,
	"month": 30 * 24 * time.Hour,
}

func (h *Incidents) Serve(w http.ResponseWriter, r *http.Request) {
	window := r.URL.Query().Get("window")
	span, ok := incidentWindows[window]
	if !ok {
		window, span = "week", 7*24*time.Hour
	}

	now := time.Now()
	from := now.Add(-span)

	incidents, err := h.Store.GetIncidents(from, now, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	names := map[string]string{}
	if mons, err := h.Store.GetMonitors(); err == nil {
		for _, m := range mons {
			names[m.ID] = m.Name
		}
	}

	perMonitor := map[string]int{}
	for _, inc := range incidents {
		perMonitor[inc.MonitorID]++
	}

	rows := make([]fleetIncident, 0, len(incidents))
	var downtime time.Duration
	ongoing := 0
	for _, inc := range incidents {
		d := inc.Duration(now)
		downtime += d
		if inc.Ongoing() {
			ongoing++
		}
		name := names[inc.MonitorID]
		if name == "" {
			name = inc.MonitorID
		}
		row := fleetIncident{
			MonitorID: inc.MonitorID,
			Name:      name,
			Started:   inc.StartedAt.Local().Format("02 Jan, 15:04"),
			Duration:  humanDuration(d),
			Reason:    inc.Reason,
			Ongoing:   inc.Ongoing(),
			Repeats:   perMonitor[inc.MonitorID],
		}
		if inc.Ongoing() {
			row.Duration = "running " + row.Duration
		}
		rows = append(rows, row)
	}

	_ = httpx.Render(w, h.TplDir, "dashboard/incidents.html", pongo2.Context{
		"version":    buildVersion(),
		"nav_active": "incidents",
		"window":     window,
		"incidents":  rows,
		"total":      len(rows),
		"ongoing":    ongoing,
		"downtime":   humanDuration(downtime),
	})
}
