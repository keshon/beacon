package server

import (
	"net/http"
)

func (s *Server) handleStreamChecks(w http.ResponseWriter, r *http.Request) {
	hub := s.deps.StreamHub
	if hub == nil {
		http.Error(w, "stream unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	if _, err := w.Write([]byte(": ok\n\n")); err != nil {
		return
	}
	flusher.Flush()

	if s.deps.Store != nil {
		state, err := s.deps.Store.GetAllState()
		if err == nil && state != nil {
			for id, st := range state {
				line := formatStateEvent(id, st)
				if len(line) == 0 {
					continue
				}
				if _, err := w.Write(line); err != nil {
					return
				}
			}
			flusher.Flush()
		}
	}

	ch, unregister := hub.Register(64)
	defer unregister()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-ch:
			if !ok {
				return
			}
			if _, err := w.Write(line); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
