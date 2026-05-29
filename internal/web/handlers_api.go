package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/keshon/beacon/internal/commands"
	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/sync"

	"github.com/keshon/commandkit"
)

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

func (s *Server) apiMonitorUptime(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	mon, err := s.store.GetMonitor(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	st, err := s.store.GetState(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if mon == nil && st == nil {
		http.NotFound(w, r)
		return
	}
	limit := 120
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			limit = n
		}
	}
	samples, err := s.store.GetUptimeSamples(id, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	type point struct {
		Time    string `json:"time"`
		Success bool   `json:"success"`
	}
	out := make([]point, 0, len(samples))
	for _, e := range samples {
		out = append(out, point{
			Time:    e.Time.UTC().Format(time.RFC3339Nano),
			Success: e.Success,
		})
	}
	s.jsonResponse(w, out)
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

func (s *Server) apiHealth(w http.ResponseWriter, r *http.Request) {
	type health struct {
		Status        string `json:"status"`
		StoreOK       bool   `json:"store_ok"`
		DroppedChecks uint64 `json:"dropped_checks"`
	}
	h := health{Status: "ok", StoreOK: true}
	if err := s.store.Ping(); err != nil {
		h.Status = "degraded"
		h.StoreOK = false
	}
	if s.scheduler != nil {
		h.DroppedChecks = s.scheduler.DroppedChecks()
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h)
}

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
