package web

import (
	"net/http"

	"github.com/keshon/beacon/internal/commands"
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
