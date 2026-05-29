package web

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/sync"
)

func (s *Server) apiSyncExport(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.Network.Enabled || s.cfg.Network.NodeID == "" {
		http.Error(w, "network not configured", http.StatusServiceUnavailable)
		return
	}
	monitors, err := s.store.GetMonitors()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	state, err := s.store.GetAllState()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
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
