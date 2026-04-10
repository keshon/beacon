package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/keshon/beacon/internal/commands"
	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/store"
	"github.com/keshon/beacon/internal/sync"

	"github.com/flosch/pongo2/v6"
	"github.com/keshon/commandkit"
)

func (s *Server) handleLoginForm(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(sessionCookie)
	if cookie != nil && s.auth.GetSession(cookie.Value) != nil {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
		return
	}
	s.render(w, "login.html", pongo2.Context{})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	user := r.FormValue("username")
	pass := r.FormValue("password")
	if user != s.cfg.Auth.Username || pass != s.cfg.Auth.Password {
		s.render(w, "login.html", pongo2.Context{"error": "Invalid credentials"})
		return
	}
	sid := s.auth.CreateSession(user)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    sid,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, _ := r.Cookie(sessionCookie); c != nil {
		s.auth.DeleteSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/login", http.StatusFound)
}

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
		r := dashboardRow{Monitor: m, State: st, Status: "unknown", SourceLabel: "This node", IsPeer: false}
		if st != nil {
			r.Status = st.Status
			if st.Latency > 0 {
				r.LatencyMs = strconv.FormatInt(st.Latency.Milliseconds(), 10) + "ms"
			}
			if !st.LastCheck.IsZero() {
				r.LastCheck = st.LastCheck.Format("15:04:05")
			}
		}
		if r.LatencyMs == "" {
			r.LatencyMs = "—"
		}
		if r.LastCheck == "" {
			r.LastCheck = "—"
		}
		rows = append(rows, r)
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
					r := dashboardRow{
						Monitor: m, State: st, SourceLabel: "Peer: " + sourceLabel,
						SourceNodeID: pd.NodeID, IsPeer: true,
					}
					r.Status = "unknown"
					if st != nil {
						r.Status = st.Status
						if st.Latency > 0 {
							r.LatencyMs = strconv.FormatInt(st.Latency.Milliseconds(), 10) + "ms"
						}
						if !st.LastCheck.IsZero() {
							r.LastCheck = st.LastCheck.Format("15:04:05")
						}
					}
					if r.LatencyMs == "" {
						r.LatencyMs = "—"
					}
					if r.LastCheck == "" {
						r.LastCheck = "—"
					}
					rows = append(rows, r)
				}
			}
		}
	}

	networkNodes := s.buildNetworkNodes()
	s.render(w, "dashboard.html", pongo2.Context{
		"version":        getBuildVersion(),
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

func (s *Server) buildNetworkNodes() []networkNode {
	var nodes []networkNode
	if !s.cfg.Network.Enabled {
		return nodes
	}
	deadTimeout := time.Duration(s.cfg.Network.DeadTimeout) * time.Second
	peerData := s.store.GetAllPeerData()
	ownMonitors := s.store.GetMonitors()

	nodes = append(nodes, networkNode{
		NodeID:        s.cfg.Network.NodeID,
		NodeIDShort:   truncateNodeID(s.cfg.Network.NodeID, 8),
		URL:           s.cfg.Network.SelfURL,
		Status:        "self",
		MonitorsCount: len(ownMonitors),
	})

	peerURLToData := make(map[string]*store.PeerData)
	for _, pd := range peerData {
		key := strings.TrimSuffix(pd.PeerURL, "/")
		if key == "" {
			key = pd.NodeID
		}
		peerURLToData[key] = pd
	}

	for _, peerURL := range s.cfg.Network.Peers {
		if peerURL == "" {
			continue
		}
		trimmed := strings.TrimSuffix(peerURL, "/")
		if trimmed == strings.TrimSuffix(s.cfg.Network.SelfURL, "/") {
			continue
		}
		pd := peerURLToData[trimmed]
		if pd == nil {
			nodes = append(nodes, networkNode{
				NodeID:        "",
				NodeIDShort:   "—",
				URL:           peerURL,
				Status:        "unknown",
				LastSeen:      "—",
				MonitorsCount: 0,
			})
			continue
		}
		status := "live"
		if time.Since(pd.LastSeen) >= deadTimeout {
			status = "dead"
		}
		lastSeen := "—"
		if !pd.LastSeen.IsZero() {
			lastSeen = formatTimeAgo(pd.LastSeen)
		}
		nodes = append(nodes, networkNode{
			NodeID:        pd.NodeID,
			NodeIDShort:   truncateNodeID(pd.NodeID, 8),
			URL:           pd.PeerURL,
			Status:        status,
			LastSeen:      lastSeen,
			MonitorsCount: len(pd.Monitors),
			LastError:     pd.LastError,
		})
	}
	return nodes
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
	s.render(w, "monitors.html", pongo2.Context{"version": getBuildVersion(), "monitors": rows})
}

func (s *Server) runCommand(w http.ResponseWriter, r *http.Request, name string) {
	cmd := commandkit.DefaultRegistry.Get(name)
	if cmd == nil {
		http.Error(w, "command not found", http.StatusNotFound)
		return
	}
	inv := &commandkit.Invocation{Data: &commands.HTTPData{W: w, R: r}}
	if err := cmd.Run(r.Context(), inv); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) runCommandWithID(w http.ResponseWriter, r *http.Request, name string, id string) {
	cmd := commandkit.DefaultRegistry.Get(name)
	if cmd == nil {
		http.Error(w, "command not found", http.StatusNotFound)
		return
	}
	inv := &commandkit.Invocation{Data: &commands.HTTPData{W: w, R: r, PathID: id}}
	if err := cmd.Run(r.Context(), inv); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) apiMonitors(w http.ResponseWriter, r *http.Request) {
	s.runCommand(w, r, "monitor:list")
}

func (s *Server) apiCreateMonitor(w http.ResponseWriter, r *http.Request) {
	s.runCommand(w, r, "monitor:add")
}

func (s *Server) apiDeleteMonitor(w http.ResponseWriter, r *http.Request) {
	s.runCommandWithID(w, r, "monitor:delete", r.PathValue("id"))
}

func (s *Server) apiUpdateMonitor(w http.ResponseWriter, r *http.Request) {
	s.runCommandWithID(w, r, "monitor:update", r.PathValue("id"))
}

func (s *Server) apiState(w http.ResponseWriter, r *http.Request) {
	s.runCommand(w, r, "state:get")
}

func (s *Server) apiEvents(w http.ResponseWriter, r *http.Request) {
	s.runCommand(w, r, "events:get")
}

func (s *Server) apiConfigGet(w http.ResponseWriter, r *http.Request) {
	s.runCommand(w, r, "config:get")
}

func (s *Server) apiConfigSet(w http.ResponseWriter, r *http.Request) {
	s.runCommand(w, r, "config:set")
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	s.render(w, "settings.html", pongo2.Context{"version": getBuildVersion()})
}

func (s *Server) handleUikit(w http.ResponseWriter, r *http.Request) {
	s.render(w, "uikit.html", pongo2.Context{"version": getBuildVersion()})
}

func (s *Server) apiHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (s *Server) apiSyncExport(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.Network.Enabled || s.cfg.Network.NodeID == "" {
		http.Error(w, "network not configured", http.StatusServiceUnavailable)
		return
	}
	monitors := s.store.GetMonitors()
	state := s.store.GetAllState()
	if state == nil {
		state = make(map[string]*monitor.MonitorState)
	}
	payload := sync.ExportPayload{
		NodeID:   s.cfg.Network.NodeID,
		Monitors: monitors,
		State:    state,
		Time:     time.Now(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}

type networkNode struct {
	NodeID        string `json:"node_id"`
	NodeIDShort   string `json:"node_id_short"`
	URL           string `json:"url"`
	Status        string `json:"status"`
	LastSeen      string `json:"last_seen,omitempty"`
	MonitorsCount int    `json:"monitors_count"`
	LastError     string `json:"last_error,omitempty"`
}

func (s *Server) apiNetworkStatus(w http.ResponseWriter, r *http.Request) {
	var nodes []networkNode
	if !s.cfg.Network.Enabled {
		s.jsonResponse(w, map[string]any{"nodes": nodes})
		return
	}
	deadTimeout := time.Duration(s.cfg.Network.DeadTimeout) * time.Second
	peerData := s.store.GetAllPeerData()
	ownMonitors := s.store.GetMonitors()

	nodes = append(nodes, networkNode{
		NodeID:        s.cfg.Network.NodeID,
		NodeIDShort:   truncateNodeID(s.cfg.Network.NodeID, 8),
		URL:           s.cfg.Network.SelfURL,
		Status:        "self",
		MonitorsCount: len(ownMonitors),
	})

	peerURLToData := make(map[string]*store.PeerData)
	for _, pd := range peerData {
		key := strings.TrimSuffix(pd.PeerURL, "/")
		if key == "" {
			key = pd.NodeID
		}
		peerURLToData[key] = pd
	}

	for _, peerURL := range s.cfg.Network.Peers {
		if peerURL == "" {
			continue
		}
		trimmed := strings.TrimSuffix(peerURL, "/")
		if trimmed == strings.TrimSuffix(s.cfg.Network.SelfURL, "/") {
			continue
		}
		pd := peerURLToData[trimmed]
		if pd == nil {
			nodes = append(nodes, networkNode{
				NodeID:        "",
				NodeIDShort:   "—",
				URL:           peerURL,
				Status:        "unknown",
				LastSeen:      "—",
				MonitorsCount: 0,
			})
			continue
		}
		status := "live"
		if time.Since(pd.LastSeen) >= deadTimeout {
			status = "dead"
		}
		lastSeen := "—"
		if !pd.LastSeen.IsZero() {
			lastSeen = formatTimeAgo(pd.LastSeen)
		}
		nodes = append(nodes, networkNode{
			NodeID:        pd.NodeID,
			NodeIDShort:   truncateNodeID(pd.NodeID, 8),
			URL:           pd.PeerURL,
			Status:        status,
			LastSeen:      lastSeen,
			MonitorsCount: len(pd.Monitors),
			LastError:     pd.LastError,
		})
	}
	s.jsonResponse(w, map[string]any{"nodes": nodes})
}

func truncateNodeID(id string, n int) string {
	if len(id) <= n {
		return id
	}
	return id[:n] + "..."
}

func formatTimeAgo(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		m := int(d.Minutes())
		if m == 1 {
			return "1 min ago"
		}
		return strconv.Itoa(m) + " min ago"
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return strconv.Itoa(h) + " hours ago"
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day ago"
	}
	return strconv.Itoa(days) + " days ago"
}
