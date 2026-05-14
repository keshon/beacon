package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/keshon/beacon/internal/store"
)

type networkNode struct {
	NodeID        string `json:"node_id"`
	NodeIDShort   string `json:"node_id_short"`
	URL           string `json:"url"`
	Status        string `json:"status"`
	LastSeen      string `json:"last_seen,omitempty"`
	MonitorsCount int    `json:"monitors_count"`
	LastError     string `json:"last_error,omitempty"`
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

func (s *Server) apiNetworkStatus(w http.ResponseWriter, r *http.Request) {
	s.jsonResponse(w, map[string]any{"nodes": s.buildNetworkNodes()})
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
