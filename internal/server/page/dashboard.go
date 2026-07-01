package page

import (
	"net/http"
	"strconv"

	"github.com/keshon/beacon/internal/cluster"
	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/server/httpx"
	"github.com/keshon/beacon/internal/storage"

	"github.com/flosch/pongo2/v6"
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
	if h.Cluster != nil {
		clusterRows, err := h.Cluster.DashboardRows(state, ownMonitors)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, cr := range clusterRows {
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

	var networkNodes any
	if h.Cluster != nil {
		nodes, _ := h.Cluster.NetworkNodes()
		networkNodes = nodes
	}

	_ = httpx.Render(w, h.TplDir, "dashboard/dashboard.html", pongo2.Context{
		"version":        buildVersion(),
		"nav_active":     "dashboard",
		"rows":           rows,
		"networkNodes":   networkNodes,
		"networkEnabled": h.Cfg.Network.Enabled,
	})
}
