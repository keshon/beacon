package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/keshon/beacon/internal/monitor"

	"github.com/flosch/pongo2/v6"
)

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
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
	state := s.store.GetAllState()
	if state == nil {
		state = make(map[string]*monitor.MonitorState)
	}

	ownMonitors := s.store.GetMonitors()
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
		peerData := s.store.GetAllPeerData()
		deadTimeout := time.Duration(s.cfg.Network.DeadTimeout) * time.Second
		for _, pd := range peerData {
			if time.Since(pd.LastSeen) < deadTimeout {
				sourceLabel := shortURL(pd.PeerURL)
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
	s.render(w, "dashboard.html", pongo2.Context{
		"version":        getBuildVersion(),
		"nav_active":     "dashboard",
		"rows":           rows,
		"networkNodes":   networkNodes,
		"networkEnabled": s.cfg.Network.Enabled,
	})
}

func shortURL(url string) string {
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimSuffix(url, "/")
	if url == "" {
		return "peer"
	}
	return url
}

func (s *Server) handleMonitors(w http.ResponseWriter, r *http.Request) {
	monitors := s.store.GetMonitors()
	type monitorRow struct {
		*monitor.Monitor
		IntervalSec    int
		TgToken        string
		TgChatID       string
		DiscordWebhook string
	}
	rows := make([]monitorRow, 0, len(monitors))
	for _, m := range monitors {
		sec := 0
		if m.Interval > 0 {
			sec = int(m.Interval / time.Second)
		}
		tgToken, tgChat, discord := "", "", ""
		if m.NotifyOverride != nil {
			if m.NotifyOverride.Telegram != nil {
				tgToken = m.NotifyOverride.Telegram.Token
				tgChat = m.NotifyOverride.Telegram.ChatID
			}
			if m.NotifyOverride.Discord != nil {
				discord = m.NotifyOverride.Discord.Webhook
			}
		}
		rows = append(rows, monitorRow{
			Monitor:        m,
			IntervalSec:    sec,
			TgToken:        tgToken,
			TgChatID:       tgChat,
			DiscordWebhook: discord,
		})
	}
	s.render(w, "monitors.html", pongo2.Context{
		"version":    getBuildVersion(),
		"nav_active": "monitors",
		"monitors":   rows,
	})
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	s.render(w, "settings.html", pongo2.Context{
		"version":    getBuildVersion(),
		"nav_active": "settings",
	})
}
