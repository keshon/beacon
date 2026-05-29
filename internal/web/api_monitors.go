package web

import (
	"net/http"
	"strconv"
	"time"
)

func (s *Server) apiMonitorList(w http.ResponseWriter, r *http.Request) {
	s.runCommand(w, r, "monitor:list")
}

func (s *Server) apiMonitorCreate(w http.ResponseWriter, r *http.Request) {
	s.runCommand(w, r, "monitor:add")
}

func (s *Server) apiMonitorDelete(w http.ResponseWriter, r *http.Request) {
	s.runCommandWithID(w, r, "monitor:delete", r.PathValue("id"))
}

func (s *Server) apiMonitorUpdate(w http.ResponseWriter, r *http.Request) {
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
	for _, rec := range samples {
		out = append(out, point{
			Time:    rec.Time.UTC().Format(time.RFC3339Nano),
			Success: rec.Success,
		})
	}
	s.jsonResponse(w, out)
}
