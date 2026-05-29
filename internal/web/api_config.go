package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/keshon/beacon/internal/service"
)

func (s *Server) apiStateGet(w http.ResponseWriter, r *http.Request) {
	st, err := service.GetAllState(s.store)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, st)
}

func (s *Server) apiCheckRecords(w http.ResponseWriter, r *http.Request) {
	limit := service.ParseRecordLimit(r.URL.Query().Get("limit"))
	records, err := service.GetCheckRecords(s.store, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, records)
}

func (s *Server) apiConfigGet(w http.ResponseWriter, r *http.Request) {
	pub, err := service.GetPublicConfig(s.store)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	pub.RequiresRestart = s.configNeedsRestart()
	s.jsonResponse(w, pub)
}

func (s *Server) apiConfigSet(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	pub, err := service.ApplyConfigPatch(s.store, s.cfg, body)
	if err != nil {
		if errors.Is(err, service.ErrInvalidJSON) {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	pub.RequiresRestart = s.configNeedsRestart()
	s.jsonResponse(w, pub)
}

func (s *Server) configNeedsRestart() bool {
	// listen and worker pool size apply only after process restart
	return true
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
