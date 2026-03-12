package web

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/store"

	"github.com/flosch/pongo2/v6"
)

type Server struct {
	store  *store.Store
	auth   *Auth
	cfg    *config.Config
	tplDir string
}

func NewServer(s *store.Store, auth *Auth, cfg *config.Config, tplDir string) *Server {
	return &Server{
		store:  s,
		auth:   auth,
		cfg:    cfg,
		tplDir: tplDir,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// Static-ish: login doesn't need auth
	mux.HandleFunc("GET /login", s.handleLoginForm)
	mux.HandleFunc("POST /login", s.handleLogin)
	mux.HandleFunc("GET /logout", s.handleLogout)

	mux.HandleFunc("GET /dashboard", s.handleDashboard)
	mux.HandleFunc("GET /monitors", s.handleMonitors)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/dashboard", http.StatusFound)
	})

	// API
	mux.HandleFunc("GET /api/monitors", s.apiMonitors)
	mux.HandleFunc("POST /api/monitors", s.apiCreateMonitor)
	mux.HandleFunc("DELETE /api/monitors/{id}", s.apiDeleteMonitor)
	mux.HandleFunc("PATCH /api/monitors/{id}", s.apiUpdateMonitor)
	mux.HandleFunc("GET /api/state", s.apiState)
	mux.HandleFunc("GET /api/events", s.apiEvents)
	mux.HandleFunc("GET /api/config", s.apiConfigGet)
	mux.HandleFunc("PUT /api/config", s.apiConfigSet)
	mux.HandleFunc("GET /api/sync/export", s.apiSyncExport)
	mux.HandleFunc("GET /api/health", s.apiHealth)
	mux.HandleFunc("GET /api/network/status", s.apiNetworkStatus)

	mux.HandleFunc("GET /settings", s.handleSettings)

	authMw := s.auth.Middleware(s.cfg.Auth.Username, s.cfg.Auth.Password)
	return authMw(mux)
}

func (s *Server) render(w http.ResponseWriter, name string, ctx pongo2.Context) error {
	tpl, err := pongo2.FromFile(filepath.Join(s.tplDir, name))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}
	return tpl.ExecuteWriter(ctx, w)
}

func (s *Server) jsonResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
