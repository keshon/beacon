package page

import (
	"net/http"
	"strconv"
	"time"

	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/server/httpx"
	"github.com/keshon/beacon/internal/storage"

	"github.com/flosch/pongo2/v6"
)

// The monitor screen.
//
// Until now there was none: a monitor could be edited in a dialog and that was
// all. Everything a person actually asks when something breaks — what happened,
// when, how long, how often, and is it getting worse — had nowhere to be shown.
//
// The screen answers in that order: the day as one strip, then availability,
// then the incidents, then the last checks. Deliberately not a form; editing is
// a different job and keeps its dialog.
type Monitor struct {
	Store  *storage.Store
	Cfg    *config.Live
	TplDir string
}

type uptimeWindow struct {
	Label   string
	Percent string
	Tone    string
	// Downtime is human-readable, because "99.86%" hides whether that was one
	// long outage or forty short ones.
	Downtime string
	Known    bool
}

type incidentRow struct {
	Started  string
	Duration string
	Reason   string
	Ongoing  bool
	Tone     string
}

type checkRow struct {
	Time    string
	Latency string
	OK      bool
	Error   string
}

func (h *Monitor) Serve(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mon, err := h.Store.GetMonitor(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if mon == nil {
		http.NotFound(w, r)
		return
	}

	now := time.Now()
	state, _ := h.Store.GetState(id)

	rollups, _ := h.Store.GetRollups(id, now.Add(-historyHours*time.Hour), now)
	history := buildHistory(rollups, now)

	monthly, _ := h.Store.GetRollups(id, now.Add(-30*24*time.Hour), now)
	windows := []uptimeWindow{
		uptimeOver(monthly, now.Add(-24*time.Hour), now, "24 hours"),
		uptimeOver(monthly, now.Add(-7*24*time.Hour), now, "7 days"),
		uptimeOver(monthly, now.Add(-30*24*time.Hour), now, "30 days"),
	}

	incidents, _ := h.Store.GetMonitorIncidents(id, now.Add(-90*24*time.Hour), now)
	rows := make([]incidentRow, 0, len(incidents))
	for _, inc := range incidents {
		row := incidentRow{
			Started:  inc.StartedAt.Local().Format("02 Jan, 15:04"),
			Duration: humanDuration(inc.Duration(now)),
			Reason:   inc.Reason,
			Ongoing:  inc.Ongoing(),
			Tone:     "error",
		}
		if inc.Ongoing() {
			row.Duration = "running " + row.Duration
		}
		rows = append(rows, row)
	}

	samples, _ := h.Store.GetUptimeSamples(id, 30)
	checks := make([]checkRow, 0, len(samples))
	for i := len(samples) - 1; i >= 0; i-- { // newest first
		rec := samples[i]
		checks = append(checks, checkRow{
			Time:    rec.Time.Local().Format("15:04:05"),
			Latency: strconv.FormatInt(rec.Latency.Milliseconds(), 10) + "ms",
			OK:      rec.Success,
			Error:   rec.Error,
		})
	}

	certText, certTone := certLine(state, now)

	mute, _ := h.Store.GetMute(id)
	mutedUntil := ""
	if mute.Active(now) {
		mutedUntil = mute.Until.Local().Format("15:04")
	}

	// An acknowledgement only exists while an incident does: the note is about
	// this outage, not about the monitor.
	ackNote, ongoing := "", false
	for _, inc := range incidents {
		if inc.Ongoing() {
			ongoing = true
			if inc.Acknowledged() {
				ackNote = inc.AckNote
				if ackNote == "" {
					ackNote = "acknowledged"
				}
			}
			break
		}
	}
	steps := buildSteps(state, mon.Target, now)

	status := monitor.StatusUnknown
	if state != nil && state.Status != "" {
		status = state.Status
	}

	_ = httpx.Render(w, h.TplDir, "dashboard/monitor/detail.html", pongo2.Context{
		"version":      buildVersion(),
		"nav_active":   "dashboard",
		"mon":          mon,
		"state":        state,
		"status":       status,
		"enabled":      mon.Enabled,
		"intervalSec":  int(mon.Interval / time.Second),
		"history":      history,
		"historyLabel": historyLabel(history),
		"windows":      windows,
		"incidents":    rows,
		"checks":       checks,
		"repeats":      countWithin(incidents, now.Add(-7*24*time.Hour)),
		"certText":     certText,
		"certTone":     certTone,
		"steps":        steps,
		"mutedUntil":   mutedUntil,
		"ongoing":      ongoing,
		"ackNote":      ackNote,
		"lastCheck":    lastCheckLabel(state),
	})
}

// uptimeOver computes availability from hourly buckets over a window.
//
// It reports Known=false when there are no buckets at all, instead of showing
// 100%. An untested monitor is not a healthy one, and a screen that cannot tell
// the difference will be believed anyway.
func uptimeOver(rollups []storage.Rollup, from, to time.Time, label string) uptimeWindow {
	lo, hi := from.UTC().Unix(), to.UTC().Unix()
	var total, failed int
	for _, r := range rollups {
		if r.Hour < lo || r.Hour > hi {
			continue
		}
		total += r.Total
		failed += r.Failed
	}
	if total == 0 {
		return uptimeWindow{Label: label, Percent: "—", Downtime: "no checks"}
	}

	ratio := float64(total-failed) / float64(total) * 100
	out := uptimeWindow{
		Label:   label,
		Percent: strconv.FormatFloat(ratio, 'f', 2, 64) + "%",
		Known:   true,
	}
	switch {
	case failed == 0:
		out.Tone = ""
		out.Downtime = "no failures"
	case ratio >= 99:
		out.Tone = "warn"
	default:
		out.Tone = "error"
	}
	if failed > 0 {
		out.Downtime = strconv.Itoa(failed) + " of " + strconv.Itoa(total) + " checks failed"
	}
	return out
}

func countWithin(incidents []storage.Incident, since time.Time) int {
	n := 0
	for _, inc := range incidents {
		if !inc.StartedAt.Before(since) {
			n++
		}
	}
	return n
}

// humanDuration prints a span the way a person says it out loud.
func humanDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return strconv.Itoa(int(d.Seconds())) + "s"
	case d < time.Hour:
		return strconv.Itoa(int(d.Minutes())) + "m"
	case d < 24*time.Hour:
		h := int(d.Hours())
		m := int(d.Minutes()) - h*60
		if m == 0 {
			return strconv.Itoa(h) + "h"
		}
		return strconv.Itoa(h) + "h " + strconv.Itoa(m) + "m"
	default:
		return strconv.Itoa(int(d.Hours()/24)) + "d"
	}
}

// lastCheckLabel names when the last check ran, or says there was none.
func lastCheckLabel(st *monitor.MonitorState) string {
	if st == nil || st.LastCheck.IsZero() {
		return "no checks yet"
	}
	return st.LastCheck.Local().Format("15:04:05")
}
