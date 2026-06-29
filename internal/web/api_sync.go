package web

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/keshon/beacon/internal/network"
	"github.com/keshon/beacon/internal/sync"
)

func (s *Server) apiSyncExport(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.Network.Enabled || s.cfg.Network.NodeID == "" {
		http.Error(w, "network not configured", http.StatusServiceUnavailable)
		return
	}
	view, err := network.BuildExportView(s.cfg, s.store)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	payload := sync.ExportPayload{
		NodeID:   s.cfg.Network.NodeID,
		Monitors: view.Monitors,
		State:    view.State,
		Time:     time.Now(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}
