package web

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/keshon/beacon/internal/service"
)

func (s *Server) apiMonitorList(w http.ResponseWriter, r *http.Request) {
	list, err := service.ListMonitors(s.store)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, list)
}

func (s *Server) apiMonitorCreate(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	m, err := service.AddMonitorFromJSON(s.store, body)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	s.jsonResponse(w, m)
}

func (s *Server) apiMonitorDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	if err := service.DeleteMonitor(s.store, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiMonitorUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	m, err := service.UpdateMonitorFromJSON(s.store, id, body)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	s.jsonResponse(w, m)
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

func writeServiceError(w http.ResponseWriter, err error) {
	if errors.Is(err, service.ErrInvalidJSON) {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if errors.Is(err, service.ErrMonitorNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	http.Error(w, err.Error(), http.StatusBadRequest)
}
