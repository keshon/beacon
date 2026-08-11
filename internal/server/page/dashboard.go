package page

import (
	"encoding/json"
	"net/http"
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
	var networkNodes any
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
		networkNodes = view.NetworkNodes
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

	const uptimeBootstrapLimit = 200
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.IsPeer || row.Monitor == nil || row.Monitor.ID == "" {
			continue
		}
		ids = append(ids, row.Monitor.ID)
	}
	type point struct {
		Time    string `json:"time"`
		Success bool   `json:"success"`
	}
	bootstrap := map[string][]point{}
	if len(ids) > 0 {
		samplesByID, err := h.Store.GetUptimeSamplesBatch(ids, uptimeBootstrapLimit)
		if err == nil && samplesByID != nil {
			for id, samples := range samplesByID {
				pts := make([]point, 0, len(samples))
				for _, rec := range samples {
					pts = append(pts, point{
						Time:    rec.Time.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
						Success: rec.Success,
					})
				}
				bootstrap[id] = pts
			}
		}
	}
	bootstrapJSON := []byte("{}")
	if b, err := json.Marshal(bootstrap); err == nil {
		bootstrapJSON = b
	}

	networkEnabled := h.Cfg.Load().Network.Enabled
	hasNetwork := false
	if networkEnabled && networkNodes != nil {
		switch nodes := networkNodes.(type) {
		case []cluster.NetworkNode:
			hasNetwork = len(nodes) > 0
		}
	}

	// Сводка. Экран монитора обязан отвечать на «всё ли цело» до того, как
	// человек начнёт читать карточки: сейчас для этого приходилось глазами
	// пройти каждую.
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
		"version":         buildVersion(),
		"nav_active":      "dashboard",
		"rows":            rows,
		"countUp":         up,
		"countDown":       down,
		"countPaused":     paused,
		"countTotal":      len(rows),
		"networkNodes":    networkNodes,
		"networkEnabled":  networkEnabled,
		"hasNetwork":      hasNetwork,
		"uptimeBootstrap": string(bootstrapJSON),
	})
}

func buildVersion() string {
	bi := buildinfo.Get()
	return bi.BuildTime + " " + bi.GoVersion + " (" + bi.Commit + ")"
}
