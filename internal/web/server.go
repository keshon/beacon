package web

import (
	"encoding/json"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/notify"
	"github.com/keshon/beacon/internal/realtime"
	"github.com/keshon/beacon/internal/scheduler"
	"github.com/keshon/beacon/internal/store"

	"github.com/flosch/pongo2/v6"
)

func init() {
	_ = mime.AddExtensionType(".css", "text/css")
}

type Server struct {
	store     *store.Store
	auth      *Auth
	cfg       *config.Config
	scheduler *scheduler.Scheduler
	hub       *realtime.Hub
	tplDir    string
	staticDir string
	testLimit *notify.RateLimiter
}

func NewServer(s *store.Store, auth *Auth, cfg *config.Config, sch *scheduler.Scheduler, tplDir, staticDir string, hub *realtime.Hub) *Server {
	return &Server{
		store:     s,
		auth:      auth,
		cfg:       cfg,
		scheduler: sch,
		hub:       hub,
		tplDir:    tplDir,
		staticDir: staticDir,
		testLimit: notify.NewRateLimiter(),
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
	mux.HandleFunc("GET /api/monitors/{id}/uptime", s.apiMonitorUptime)
	mux.HandleFunc("GET /api/stream/checks", s.handleStreamChecks)
	mux.HandleFunc("GET /api/state", s.apiState)
	mux.HandleFunc("GET /api/events", s.apiEvents)
	mux.HandleFunc("GET /api/config", s.apiConfigGet)
	mux.HandleFunc("PUT /api/config", s.apiConfigSet)
	mux.HandleFunc("POST /api/notify/test", s.apiNotifyTest)
	mux.HandleFunc("GET /api/notify/defaults", s.apiNotifyDefaults)
	mux.HandleFunc("GET /api/sync/export", s.apiSyncExport)
	mux.HandleFunc("GET /api/health", s.apiHealth)
	mux.HandleFunc("GET /api/network/status", s.apiNetworkStatus)

	mux.HandleFunc("GET /settings", s.handleSettings)

	checkPassword := func(user, pass string) bool {
		if user != s.cfg.Auth.Username {
			return false
		}
		return s.cfg.Auth.CheckPassword(pass)
	}
	authMw := s.auth.Middleware(s.cfg.Auth.Username, checkPassword)
	h := authMw(mux)
	if s.staticDir != "" {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/static/") {
				s.serveStatic(w, r)
				return
			}
			h.ServeHTTP(w, r)
		})
	}
	return h
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
