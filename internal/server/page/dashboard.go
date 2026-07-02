package page

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

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
	Cfg     *config.Config
	Cluster *cluster.Runtime
	TplDir  string
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

	const uptimeLimit = 45
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
		samplesByID, err := h.Store.GetUptimeSamplesBatch(ids, uptimeLimit)
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

	view := strings.TrimSpace(r.URL.Query().Get("view"))
	if view != "list" && view != "table" && view != "cards" {
		view = "cards"
	}

	_ = httpx.Render(w, h.TplDir, "dashboard/dashboard.html", pongo2.Context{
		"version":         buildVersion(),
		"nav_active":      "dashboard",
		"rows":            rows,
		"networkNodes":    networkNodes,
		"networkEnabled":  h.Cfg.Network.Enabled,
		"uptimeBootstrap": string(bootstrapJSON),
		"dashboardView":   view,
	})
}

func buildVersion() string {
	bi := buildinfo.Get()
	return bi.BuildTime + " " + bi.GoVersion + " (" + bi.Commit + ")"
}
