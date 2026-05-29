package web

import "net/http"

func (s *Server) apiStreamChecks(w http.ResponseWriter, r *http.Request) {
	if s.streamHub == nil {
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

	ch, unregister := s.streamHub.Register(16)
	defer unregister()

	if _, err := w.Write([]byte(": ok\n\n")); err != nil {
		return
	}
	flusher.Flush()

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
