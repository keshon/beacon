package page

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/keshon/beacon/internal/cluster"
	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/server/httpx"
	"github.com/keshon/beacon/internal/storage"

	"github.com/flosch/pongo2/v6"
	"github.com/keshon/buildinfo"
)

type Dashboard struct {
	Store   *storage.Store
	Cfg     *config.Live
	Cluster *cluster.Runtime
	TplDir  string
}

type dashboardRow struct {
	Monitor      *monitor.Monitor
	State        *monitor.MonitorState
	LatencyMs    string
	LastCheck    string
	Status       string
	SourceLabel  string
	SourceNodeID string
	IsPeer       bool
	Adopted      bool
	IntervalSec  int
	NotifyJSON   string
	HTTPJSON     string
	Enabled      bool
	ConfigJSON   string

	History      []HistTick
	HistoryLabel string
	// Repeats is how many incidents this monitor had in the last week. One
	// outage is an event; the third in a week is a cause, and the row is the
	// only place a person will notice the difference.
	Repeats int
}

func enrichDashboardRowFromMonitor(row *dashboardRow, m *monitor.Monitor) {
	if row == nil || m == nil {
		return
	}
	row.Enabled = m.Enabled
	if m.Interval > 0 {
		row.IntervalSec = int(m.Interval / time.Second)
	}
	notifyJSON := "{}"
	if m.NotifyOverride != nil {
		if buf, err := json.Marshal(m.NotifyOverride); err == nil {
			notifyJSON = string(buf)
		}
	}
	row.NotifyJSON = notifyJSON
	httpJSON := "{}"
	if m.HTTP != nil {
		if buf, err := json.Marshal(m.HTTP.Redacted()); err == nil {
			httpJSON = string(buf)
		}
	}
	row.HTTPJSON = httpJSON
	cfg := struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Type        string `json:"type"`
		Target      string `json:"target"`
		IntervalSec int    `json:"interval_sec"`
		Enabled     bool   `json:"enabled"`
		NotifyJSON  string `json:"notify_json"`
		HTTPJSON    string `json:"http_json"`
	}{
		ID:          m.ID,
		Name:        m.Name,
		Type:        m.Type,
		Target:      m.Target,
		IntervalSec: row.IntervalSec,
		Enabled:     m.Enabled,
		NotifyJSON:  notifyJSON,
		HTTPJSON:    httpJSON,
	}
	if buf, err := json.Marshal(cfg); err == nil {
		row.ConfigJSON = string(buf)
	}
}

func (h *Dashboard) Serve(w http.ResponseWriter, r *http.Request) {
	state, err := h.Store.GetAllState()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if state == nil {
		state = make(map[string]*monitor.MonitorState)
	}

	ownMonitors, err := h.Store.GetMonitors()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ownByID := make(map[string]*monitor.Monitor, len(ownMonitors))
	for _, m := range ownMonitors {
		ownByID[m.ID] = m
	}

	var rows []dashboardRow
	if h.Cluster != nil {
		view, err := h.Cluster.DashboardView(state, ownMonitors)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, cr := range view.Rows {
			rows = append(rows, dashboardRow{
				Monitor: cr.Monitor, State: cr.State, LatencyMs: cr.LatencyMs,
				LastCheck: cr.LastCheck, Status: cr.Status, SourceLabel: cr.SourceLabel,
				SourceNodeID: cr.SourceNodeID, IsPeer: cr.IsPeer, Adopted: cr.Adopted,
			})
		}
	} else {
		for _, m := range ownMonitors {
			st := state[m.ID]
			row := dashboardRow{Monitor: m, State: st, Status: "unknown", SourceLabel: "This node"}
			if st != nil {
				row.Status = st.Status
				if st.Latency > 0 {
					row.LatencyMs = strconv.FormatInt(st.Latency.Milliseconds(), 10) + "ms"
				}
				if !st.LastCheck.IsZero() {
					row.LastCheck = st.LastCheck.Format("15:04:05")
				}
			}
			if row.LatencyMs == "" {
				row.LatencyMs = "—"
			}
			if row.LastCheck == "" {
				row.LastCheck = "—"
			}
			rows = append(rows, row)
		}
	}

	for i := range rows {
		if rows[i].IsPeer || rows[i].Monitor == nil || rows[i].Monitor.ID == "" {
			continue
		}
		if m, ok := ownByID[rows[i].Monitor.ID]; ok {
			enrichDashboardRowFromMonitor(&rows[i], m)
		}
	}

	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.IsPeer || row.Monitor == nil || row.Monitor.ID == "" {
			continue
		}
		ids = append(ids, row.Monitor.ID)
	}

	now := time.Now()
	win := windowByKey(r.URL.Query().Get("window"))
	rollups, _ := h.Store.GetRollupsBatch(ids, now.Add(-win.Span), now)
	repeats, _ := h.Store.CountIncidentsBatch(ids, now.Add(-7*24*time.Hour))
	for i := range rows {
		if rows[i].IsPeer || rows[i].Monitor == nil || rows[i].Monitor.ID == "" {
			continue
		}
		id := rows[i].Monitor.ID
		rows[i].History = buildWindowHistory(rollups[id], win, now)
		rows[i].HistoryLabel = historyLabel(rows[i].History)
		rows[i].Repeats = repeats[id]
	}

	// Anything with marks on it comes first. Alphabetical order is fine for
	// finding a monitor you already have in mind, and useless for the question
	// the screen is actually open for.
	sortRowsByAttention(rows)

	outage := buildOutage(h.Store, rows, now)

	names := make(map[string]string, len(rows))
	for _, r := range rows {
		if !r.IsPeer && r.Monitor != nil {
			names[r.Monitor.ID] = r.Monitor.Name
		}
	}
	cert := buildCertNote(state, names, now)

	// Summary. The dashboard must answer "is anything broken" before a
	// person starts reading cards; until now that answer required scanning
	// every one of them by eye.
	var up, down, paused int
	for _, r := range rows {
		switch {
		case !r.IsPeer && !r.Enabled:
			paused++
		case r.Status == "up":
			up++
		case r.Status == "down":
			down++
		}
	}

	_ = httpx.Render(w, h.TplDir, "dashboard/dashboard.html", pongo2.Context{
		"version":     buildVersion(),
		"nav_active":  "dashboard",
		"rows":        rows,
		"countUp":     up,
		"countDown":   down,
		"countPaused": paused,
		"countTotal":  len(rows),
		"window":      win.Key,
		"windowLabel": win.Label,
		"windows":     histWindows,
		"fleetTone":   fleetTone(down, up),
		"fleetLabel":  fleetLabel(down, up, paused),
		"outage":      outage,
		"cert":        cert,
	})
}

func buildVersion() string {
	bi := buildinfo.Get()
	return bi.BuildTime + " " + bi.GoVersion + " (" + bi.Commit + ")"
}

// sortRowsByAttention puts what needs looking at on top: down first, then
// anything with failures in the window, then the rest by name.
func sortRowsByAttention(rows []dashboardRow) {
	rank := func(r dashboardRow) int {
		if r.Status == "down" {
			return 0
		}
		for _, t := range r.History {
			if t.Tone != "" {
				return 1
			}
		}
		return 2
	}
	sort.SliceStable(rows, func(i, j int) bool {
		ri, rj := rank(rows[i]), rank(rows[j])
		if ri != rj {
			return ri < rj
		}
		if rows[i].Monitor == nil || rows[j].Monitor == nil {
			return false
		}
		return rows[i].Monitor.Name < rows[j].Monitor.Name
	})
}

// fleetTone and fleetLabel are the one summary the screen keeps. The row of
// four metrics went: "Monitors 17" is not an action, and an aggregate is worse
// than the list it summarises when the list is right there.
func fleetTone(down, up int) string {
	if down > 0 {
		return "error"
	}
	if up == 0 {
		return "neutral"
	}
	return "ok"
}

func fleetLabel(down, up, paused int) string {
	switch {
	case down == 1:
		return "1 not responding"
	case down > 1:
		return strconv.Itoa(down) + " not responding"
	case up == 0 && paused > 0:
		return "all paused"
	case up == 0:
		return "nothing to check yet"
	default:
		return "all " + strconv.Itoa(up) + " responding"
	}
}

// outageBlock is what stands above the list when something is down.
//
// It answers three questions in order, and the order is the point: WHAT is not
// working, WHAT WAS ALREADY TRIED, and WHAT TO DO NOW. A red badge answers only
// the first and leaves the reader guessing whether anyone has looked yet.
type outageBlock struct {
	Head   string
	Reason string
	Tried  []string
	First  string
}

func buildOutage(store *storage.Store, rows []dashboardRow, now time.Time) *outageBlock {
	var downRows []dashboardRow
	for _, r := range rows {
		if r.Status == "down" && !r.IsPeer && r.Monitor != nil {
			downRows = append(downRows, r)
		}
	}
	if len(downRows) == 0 {
		return nil
	}

	names := make([]string, 0, len(downRows))
	for _, r := range downRows {
		names = append(names, r.Monitor.Name)
	}

	out := &outageBlock{First: downRows[0].Monitor.ID}
	switch len(names) {
	case 1:
		out.Head = names[0] + " is not responding"
	case 2:
		out.Head = names[0] + " and " + names[1] + " are not responding"
	default:
		out.Head = strconv.Itoa(len(names)) + " monitors are not responding, including " + names[0]
	}

	// Facts only, and only the ones actually recorded. An empty line is better
	// than a plausible one: this block is read when trust matters most.
	first := downRows[0]
	if inc, err := store.GetMonitorIncidents(first.Monitor.ID, now.Add(-90*24*time.Hour), now); err == nil {
		for _, i := range inc {
			if !i.Ongoing() {
				continue
			}
			out.Reason = i.Reason
			if i.Checks > 1 {
				out.Tried = append(out.Tried, strconv.Itoa(i.Checks)+
					" checks in a row failed since "+i.StartedAt.Local().Format("15:04"))
			}
			break
		}
		week := 0
		for _, i := range inc {
			if !i.StartedAt.Before(now.Add(-7 * 24 * time.Hour)) {
				week++
			}
		}
		if week > 2 {
			out.Tried = append(out.Tried, strconv.Itoa(week)+
				" outages of "+first.Monitor.Name+" this week — it repeats")
		}
	}
	if first.Monitor.Retries > 0 {
		out.Tried = append(out.Tried, "declared down only after "+
			strconv.Itoa(first.Monitor.Retries)+" retries, so a single blip is not it")
	}
	return out
}
