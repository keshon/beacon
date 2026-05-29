package web

import (
	"encoding/json"
	"net/http"
)

func (s *Server) apiStateGet(w http.ResponseWriter, r *http.Request) {
	s.runCommand(w, r, "state:get")
}

func (s *Server) apiCheckRecords(w http.ResponseWriter, r *http.Request) {
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
