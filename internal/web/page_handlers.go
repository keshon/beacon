package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/keshon/beacon/internal/monitor"

	"github.com/flosch/pongo2/v6"
)

func (s *Server) pageDashboard(w http.ResponseWriter, r *http.Request) {
	type dashboardRow struct {
		Monitor      *monitor.Monitor
		State        *monitor.MonitorState
		LatencyMs    string
		LastCheck    string
		Status       string
		SourceLabel  string
		SourceNodeID string
		IsPeer       bool
	}
	var rows []dashboardRow
	state, err := s.store.GetAllState()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if state == nil {
		state = make(map[string]*monitor.MonitorState)
	}

	ownMonitors, err := s.store.GetMonitors()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, m := range ownMonitors {
		st := state[m.ID]
		row := dashboardRow{Monitor: m, State: st, Status: "unknown", SourceLabel: "This node", IsPeer: false}
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

	if s.cfg.Network.Enabled {
		peerData, err := s.store.GetAllPeerData()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		deadTimeout := time.Duration(s.cfg.Network.DeadTimeout) * time.Second
		for _, pd := range peerData {
			if time.Since(pd.LastSeen) < deadTimeout {
				sourceLabel := peerDisplayName(pd.PeerURL)
				for _, m := range pd.Monitors {
					st := pd.State[m.ID]
					if st == nil {
						st = state[m.ID]
					}
					row := dashboardRow{
						Monitor: m, State: st, SourceLabel: "Peer: " + sourceLabel,
						SourceNodeID: pd.NodeID, IsPeer: true,
					}
					row.Status = "unknown"
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
		}
	}

	networkNodes := s.buildNetworkNodes()
	s.render(w, "dashboard/dashboard.html", pongo2.Context{
		"version":        buildVersion(),
		"nav_active":     "dashboard",
		"rows":           rows,
		"networkNodes":   networkNodes,
		"networkEnabled": s.cfg.Network.Enabled,
	})
}

func peerDisplayName(url string) string {
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimSuffix(url, "/")
	if url == "" {
		return "peer"
	}
	return url
}

func (s *Server) pageMonitors(w http.ResponseWriter, r *http.Request) {
	monitors, err := s.store.GetMonitors()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	type monitorRow struct {
		*monitor.Monitor
		IntervalSec int
		NotifyJSON  string
		HTTPJSON    string
	}
	rows := make([]monitorRow, 0, len(monitors))
	for _, m := range monitors {
		sec := 0
		if m.Interval > 0 {
			sec = int(m.Interval / time.Second)
		}
		notifyJSON := "{}"
		if m.NotifyOverride != nil {
			buf, _ := json.Marshal(m.NotifyOverride)
			notifyJSON = string(buf)
		}
		httpJSON := "{}"
		if m.HTTP != nil {
			if buf, err := json.Marshal(m.HTTP.Redacted()); err == nil {
				httpJSON = string(buf)
			}
		}
		rows = append(rows, monitorRow{
			Monitor:     m,
			IntervalSec: sec,
			NotifyJSON:  notifyJSON,
			HTTPJSON:    httpJSON,
		})
	}
	s.render(w, "monitors/monitors.html", pongo2.Context{
		"version":    buildVersion(),
		"nav_active": "monitors",
		"monitors":   rows,
	})
}

func (s *Server) pageSettings(w http.ResponseWriter, r *http.Request) {
	s.render(w, "settings/settings.html", pongo2.Context{
		"version":    buildVersion(),
		"nav_active": "settings",
	})
}
