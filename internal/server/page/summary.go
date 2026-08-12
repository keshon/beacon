package page

import (
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/server/httpx"
	"github.com/keshon/beacon/internal/storage"

	"github.com/flosch/pongo2/v6"
)

// The summary screen.
//
// It answers one question — WHAT TO FIX FIRST — and it answers it by ranking,
// not by averaging. There is no fleet-wide availability figure here on purpose:
// averaging a CDN with a staging box produces a number that is true of nothing
// and reassuring about everything.
//
// Two horizons meet on this screen and they must not be confused. Hourly
// buckets go back ninety days; incidents only exist from the day the store
// learned to keep them. Each block says which one it is standing on.
type Summary struct {
	Store  *storage.Store
	Cfg    *config.Live
	TplDir string
}

type summaryWindow struct {
	Key   string
	Label string
	Span  time.Duration
}

var summaryWindows = []summaryWindow{
	{Key: "week", Label: "7 days", Span: 7 * 24 * time.Hour},
	{Key: "month", Label: "30 days", Span: 30 * 24 * time.Hour},
	{Key: "quarter", Label: "90 days", Span: 90 * 24 * time.Hour},
}

type worstRow struct {
	ID     string
	Name   string
	Failed int
	// Ratio is availability as a number, kept beside the formatted Percent so
	// the ordering and the printed column cannot drift apart.
	Ratio     float64
	Total     int
	Percent   string
	Tone      string
	History   []HistTick
	Incidents int
}

type certRow struct {
	ID    string
	Name  string
	Left  string
	Until string
	Tone  string
}

func (h *Summary) Serve(w http.ResponseWriter, r *http.Request) {
	win := summaryWindows[1]
	for _, c := range summaryWindows {
		if c.Key == r.URL.Query().Get("window") {
			win = c
		}
	}

	now := time.Now()
	from := now.Add(-win.Span)

	monitors, err := h.Store.GetMonitors()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ids := make([]string, 0, len(monitors))
	names := make(map[string]string, len(monitors))
	for _, m := range monitors {
		ids = append(ids, m.ID)
		names[m.ID] = m.Name
	}

	rollups, _ := h.Store.GetRollupsBatch(ids, from, now)
	tick := histWindow{Key: win.Key, Label: win.Label, Span: win.Span,
		PerTick: win.Span / tickCount}

	var worst []worstRow
	clean, tracked := 0, 0
	for _, m := range monitors {
		list := rollups[m.ID]
		total, failed := 0, 0
		for _, rr := range list {
			total += rr.Total
			failed += rr.Failed
		}
		if total == 0 {
			continue // nothing measured in the window: not clean, just unknown
		}
		tracked++
		if failed == 0 {
			clean++
			continue // the table is about what went wrong
		}
		ratio := float64(total-failed) / float64(total) * 100
		row := worstRow{
			ID: m.ID, Name: m.Name, Failed: failed, Total: total, Ratio: ratio,
			Percent: strconv.FormatFloat(ratio, 'f', 2, 64) + "%",
			History: buildWindowHistory(list, tick, now),
			Tone:    "warn",
		}
		if ratio < 99 {
			row.Tone = "error"
		}
		worst = append(worst, row)
	}

	// Ranked by the column the reader is looking at.
	//
	// This used to rank by the ABSOLUTE number of failed checks, and on real
	// data that put the worst monitor last: something available 78.95% of the
	// time sat at the bottom because it is checked once an hour, under a
	// monitor at 99.04% that is checked twice a minute. A screen headed "what
	// to fix first" was answering "what was checked most often".
	//
	// Sorting by the visible percentage also keeps the order EXPLICABLE: any
	// hidden statistic — a confidence bound, a weighting by sample size —
	// makes a correctly sorted table look broken. How much evidence is behind
	// each row is right there in "4 of 19"; the reader can weigh it.
	sort.Slice(worst, func(i, j int) bool {
		if worst[i].Ratio != worst[j].Ratio {
			return worst[i].Ratio < worst[j].Ratio
		}
		if worst[i].Failed != worst[j].Failed {
			return worst[i].Failed > worst[j].Failed
		}
		return worst[i].Name < worst[j].Name
	})

	incidents, _ := h.Store.GetIncidents(from, now, 0)
	var durations []time.Duration
	for i := range incidents {
		durations = append(durations, incidents[i].Duration(now))
		for j := range worst {
			if worst[j].ID == incidents[i].MonitorID {
				worst[j].Incidents++
			}
		}
	}

	states, _ := h.Store.GetAllState()
	certs := buildCertRows(states, names, now)

	_ = httpx.Render(w, h.TplDir, "dashboard/summary.html", pongo2.Context{
		"version":     buildVersion(),
		"nav_active":  "summary",
		"window":      win.Key,
		"windowLabel": win.Label,
		"windows":     summaryWindows,
		"worst":       worst,
		"certs":       certs,
		"clean":       clean,
		"tracked":     tracked,
		"incidents":   len(incidents),
		"median":      medianLabel(durations),
	})
}

// medianLabel is the middle incident, not the mean: one outage that lasted a
// day would drag an average past every other number on the screen.
func medianLabel(list []time.Duration) string {
	if len(list) == 0 {
		return "—"
	}
	sort.Slice(list, func(i, j int) bool { return list[i] < list[j] })
	return humanDuration(list[len(list)/2])
}

// buildCertRows lists every deadline it knows, nearest first. Here the full
// list belongs: the screen is read when planning, not when firefighting.
func buildCertRows(states map[string]*monitor.MonitorState, names map[string]string, now time.Time) []certRow {
	type row struct {
		certRow
		at time.Time
	}
	var rows []row
	for id, st := range states {
		if st == nil || st.CertExpiry.IsZero() {
			continue
		}
		name := names[id]
		if name == "" {
			continue
		}
		left := st.CertExpiry.Sub(now)
		item := certRow{
			ID: id, Name: name,
			Until: st.CertExpiry.Local().Format("2 Jan 2006"),
			Left:  humanDuration(left),
		}
		switch {
		case left <= 0:
			item.Left, item.Tone = "expired", "error"
		case left <= certWarnWithin:
			item.Tone = "warn"
		}
		rows = append(rows, struct {
			certRow
			at time.Time
		}{item, st.CertExpiry})
	}
	// By date, not by its printed form: "2 Jan" sorts before "3 Feb" as text
	// only by luck.
	sort.Slice(rows, func(i, j int) bool { return rows[i].at.Before(rows[j].at) })
	out := make([]certRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.certRow)
	}
	return out
}
