package server

import (
	"mime"
	"net/http"
	"strings"

	"github.com/keshon/beacon/internal/cluster"
	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor/scheduler"
	"github.com/keshon/beacon/internal/notify"
	"github.com/keshon/beacon/internal/server/api"
	"github.com/keshon/beacon/internal/server/middleware"
	"github.com/keshon/beacon/internal/server/page"
	"github.com/keshon/beacon/internal/storage"
)

type Deps struct {
	Store     *storage.Store
	Cfg       *config.Live
	Scheduler *scheduler.Scheduler
	Cluster   *cluster.Runtime
	StreamHub *CheckStreamHub
	Auth      *middleware.Auth
	TplDir    string
	StaticDir string
	TestLimit *notify.RateLimiter
}

func init() {
	_ = mime.AddExtensionType(".css", "text/css")
}

// Auth is the web session authenticator.
type Auth = middleware.Auth

// NewAuth creates a new session store.
func NewAuth() *middleware.Auth {
	return middleware.NewAuth()
}

type Server struct {
	deps Deps
}

func NewServer(s *storage.Store, auth *middleware.Auth, cfg *config.Live, sch *scheduler.Scheduler, clusterRT *cluster.Runtime, tplDir, staticDir string, hub *CheckStreamHub) *Server {
	return &Server{
		deps: Deps{
			Store:     s,
			Auth:      auth,
			Cfg:       cfg,
			Scheduler: sch,
			Cluster:   clusterRT,
			StreamHub: hub,
			TplDir:    tplDir,
			StaticDir: staticDir,
			TestLimit: notify.NewRateLimiter(),
		},
	}
}

func (s *Server) Routes() http.Handler {
	d := s.deps
	mux := http.NewServeMux()

	authPages := &page.Auth{Auth: d.Auth, Cfg: d.Cfg, TplDir: d.TplDir}
	mux.HandleFunc("GET /login", authPages.LoginForm)
	mux.HandleFunc("POST /login", authPages.Login)
	mux.HandleFunc("GET /logout", authPages.Logout)

	dashboard := &page.Dashboard{Store: d.Store, Cfg: d.Cfg, Cluster: d.Cluster, TplDir: d.TplDir}
	mux.HandleFunc("GET /dashboard", dashboard.Serve)

	monitorsPage := &page.Monitors{TplDir: d.TplDir}
	mux.HandleFunc("GET /monitors", monitorsPage.Serve)

	monitorPage := &page.Monitor{Store: d.Store, Cfg: d.Cfg, TplDir: d.TplDir}
	mux.HandleFunc("GET /monitors/{id}", monitorPage.Serve)

	incidentsPage := &page.Incidents{Store: d.Store, Cfg: d.Cfg, TplDir: d.TplDir}
	mux.HandleFunc("GET /incidents", incidentsPage.Serve)

	summaryPage := &page.Summary{Store: d.Store, Cfg: d.Cfg, Cluster: d.Cluster, TplDir: d.TplDir}
	mux.HandleFunc("GET /summary", summaryPage.Serve)

	peersPage := &page.Peers{Store: d.Store, Cfg: d.Cfg, Cluster: d.Cluster, TplDir: d.TplDir}
	mux.HandleFunc("GET /peers", peersPage.Serve)

	notificationsPage := &page.Notifications{Store: d.Store, Cfg: d.Cfg, TplDir: d.TplDir}
	mux.HandleFunc("GET /notifications", notificationsPage.Serve)

	settingsPage := &page.Settings{TplDir: d.TplDir}
	mux.HandleFunc("GET /settings", settingsPage.Serve)

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/dashboard", http.StatusFound)
	})

	monAPI := &api.Monitors{Store: d.Store}
	mux.HandleFunc("GET /api/monitors", monAPI.List)
	mux.HandleFunc("POST /api/monitors", monAPI.Create)
	mux.HandleFunc("DELETE /api/monitors/{id}", monAPI.Delete)
	mux.HandleFunc("PATCH /api/monitors/{id}", monAPI.Update)
	mux.HandleFunc("GET /api/monitors/{id}/uptime", monAPI.Uptime)

	// The reverse channel: what a person can say back to the machine.
	actionsAPI := &api.Actions{Store: d.Store, Scheduler: d.Scheduler, Cluster: d.Cluster}
	mux.HandleFunc("POST /api/monitors/{id}/check", actionsAPI.CheckNow)
	mux.HandleFunc("POST /api/monitors/{id}/mute", actionsAPI.Mute)
	mux.HandleFunc("POST /api/monitors/{id}/ack", actionsAPI.Acknowledge)
	mux.HandleFunc("POST /api/peers/sync", actionsAPI.SyncPeers)
	mux.HandleFunc("GET /api/uptime", monAPI.UptimeBatch)

	cfgAPI := &api.Config{Store: d.Store, Cfg: d.Cfg, Cluster: d.Cluster}
	mux.HandleFunc("GET /api/config", cfgAPI.Get)
	mux.HandleFunc("PUT /api/config", cfgAPI.Set)

	stateAPI := &api.State{Store: d.Store, Scheduler: d.Scheduler}
	mux.HandleFunc("GET /api/state", stateAPI.Get)
	mux.HandleFunc("GET /api/check-records", stateAPI.CheckRecords)
	mux.HandleFunc("GET /api/health", stateAPI.Health)

	notifyAPI := &api.Notify{Cfg: d.Cfg, TestLimit: d.TestLimit}
	mux.HandleFunc("POST /api/notify/test", notifyAPI.Test)
	mux.HandleFunc("GET /api/notify/defaults", notifyAPI.Defaults)

	mux.HandleFunc("GET /api/stream/checks", s.handleStreamChecks)

	if d.Cluster != nil {
		mux.HandleFunc("GET /api/sync/export", d.Cluster.HandleExport)
		mux.HandleFunc("GET /api/network/status", d.Cluster.HandleNetworkStatus)
	}

	checkPassword := func(user, pass string) bool {
		cfg := d.Cfg.Load()
		if user != cfg.Auth.Username {
			return false
		}
		return cfg.Auth.CheckPassword(pass)
	}
	username := func() string { return d.Cfg.Load().Auth.Username }
	authMw := d.Auth.Middleware(username, checkPassword, func() string { return d.Cfg.Load().Network.SyncToken })
	h := d.Auth.CSRFMiddleware()(authMw(mux))
	if d.StaticDir != "" {
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
