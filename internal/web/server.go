package web

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/keshon/beacon/internal/cluster"
	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/notify"
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
	cluster   *cluster.Runtime
	streamHub *CheckStreamHub
	tplDir    string
	staticDir string
	testLimit *notify.RateLimiter
}

func NewServer(s *store.Store, auth *Auth, cfg *config.Config, sch *scheduler.Scheduler, clusterRT *cluster.Runtime, tplDir, staticDir string, hub *CheckStreamHub) *Server {
	return &Server{
		store:     s,
		auth:      auth,
		cfg:       cfg,
		scheduler: sch,
		cluster:   clusterRT,
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
	mux.HandleFunc("GET /api/health", s.apiHealth)

	if s.cluster != nil {
		mux.HandleFunc("GET /api/sync/export", s.cluster.HandleExport)
		mux.HandleFunc("GET /api/network/status", s.cluster.HandleNetworkStatus)
	}

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

func (s *Server) apiStateGet(w http.ResponseWriter, r *http.Request) {
	st, err := s.store.GetAllState()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, st)
}

func (s *Server) apiCheckRecords(w http.ResponseWriter, r *http.Request) {
	limit := parseRecordLimit(r.URL.Query().Get("limit"))
	records, err := s.store.GetCheckRecords(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, records)
}

func (s *Server) apiConfigGet(w http.ResponseWriter, r *http.Request) {
	pub, err := getSettingsConfig(s.store)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	pub.RequiresRestart = false
	s.jsonResponse(w, pub)
}

func (s *Server) apiConfigSet(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	before := *s.cfg
	pub, err := applyConfigPatch(s.store, s.cfg, body)
	if err != nil {
		if err == errInvalidJSON {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if s.scheduler != nil {
		s.scheduler.ReloadConfig(s.cfg)
	}
	if s.cluster != nil {
		s.cluster.NotifyConfigChange()
	}
	pub.RequiresRestart = configNeedsRestart(&before, s.cfg)
	s.jsonResponse(w, pub)
}

func configNeedsRestart(before, after *config.Config) bool {
	if before == nil || after == nil {
		return false
	}
	return before.Listen != after.Listen || before.Workers != after.Workers
}

func getSettingsConfig(st *store.Store) (config.PublicConfig, error) {
	var cfg config.Config
	ok, err := st.GetConfig(&cfg)
	if err != nil {
		return config.PublicConfig{}, err
	}
	if !ok {
		cfg = *config.Default()
	}
	cfg.Normalize()
	return cfg.ToSettings(), nil
}

func applyConfigPatch(st *store.Store, runtime *config.Config, body []byte) (config.PublicConfig, error) {
	var incoming config.Config
	if err := json.Unmarshal(body, &incoming); err != nil {
		return config.PublicConfig{}, errInvalidJSON
	}
	existing := *runtime
	ok, err := st.GetConfig(&existing)
	if err != nil {
		return config.PublicConfig{}, err
	}
	if !ok {
		existing = *config.Default()
	}
	config.ApplyNonSecret(&existing, &incoming)
	if err := config.MergeSecrets(&existing, &incoming); err != nil {
		return config.PublicConfig{}, err
	}
	existing.Normalize()
	if err := existing.Auth.EnsureHashed(); err != nil {
		return config.PublicConfig{}, err
	}
	if err := st.SetConfig(&existing); err != nil {
		return config.PublicConfig{}, err
	}
	*runtime = existing
	return existing.ToSettings(), nil
}

func parseRecordLimit(s string) int {
	if s == "" {
		return 100
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 100
	}
	return n
}
