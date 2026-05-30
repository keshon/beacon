package web

import (
	"encoding/json"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/notify"
	"github.com/keshon/beacon/internal/sse"
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
	streamHub *sse.CheckStreamHub
	tplDir    string
	staticDir string
	testLimit *notify.RateLimiter
}

func NewServer(s *store.Store, auth *Auth, cfg *config.Config, sch *scheduler.Scheduler, tplDir, staticDir string, hub *sse.CheckStreamHub) *Server {
	return &Server{
		store:     s,
		auth:      auth,
		cfg:       cfg,
		scheduler: sch,
		streamHub: hub,
		tplDir:    tplDir,
		staticDir: staticDir,
		testLimit: notify.NewRateLimiter(),
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /login", s.pageLoginForm)
	mux.HandleFunc("POST /login", s.pageLogin)
	mux.HandleFunc("GET /logout", s.pageLogout)

	mux.HandleFunc("GET /dashboard", s.pageDashboard)
	mux.HandleFunc("GET /monitors", s.pageMonitors)
	mux.HandleFunc("GET /settings", s.pageSettings)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/dashboard", http.StatusFound)
	})

	mux.HandleFunc("GET /api/monitors", s.apiMonitorList)
	mux.HandleFunc("POST /api/monitors", s.apiMonitorCreate)
	mux.HandleFunc("DELETE /api/monitors/{id}", s.apiMonitorDelete)
	mux.HandleFunc("PATCH /api/monitors/{id}", s.apiMonitorUpdate)
	mux.HandleFunc("GET /api/monitors/{id}/uptime", s.apiMonitorUptime)
	mux.HandleFunc("GET /api/stream/checks", s.apiStreamChecks)
	mux.HandleFunc("GET /api/state", s.apiStateGet)
	mux.HandleFunc("GET /api/check-records", s.apiCheckRecords)
	mux.HandleFunc("GET /api/config", s.apiConfigGet)
	mux.HandleFunc("PUT /api/config", s.apiConfigSet)
	mux.HandleFunc("POST /api/notify/test", s.apiNotifyTest)
	mux.HandleFunc("GET /api/notify/defaults", s.apiNotifyDefaults)
	mux.HandleFunc("GET /api/sync/export", s.apiSyncExport)
	mux.HandleFunc("GET /api/health", s.apiHealth)
	mux.HandleFunc("GET /api/network/status", s.apiNetworkStatus)

	checkPassword := func(user, pass string) bool {
		if user != s.cfg.Auth.Username {
			return false
		}
		return s.cfg.Auth.CheckPassword(pass)
	}
	authMw := s.auth.Middleware(s.cfg.Auth.Username, checkPassword, func() string { return s.cfg.Network.SyncToken })
	h := s.auth.CSRFMiddleware()(authMw(mux))
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
