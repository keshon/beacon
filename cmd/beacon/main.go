package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/keshon/beacon/internal/checks"
	"github.com/keshon/beacon/internal/cli"
	"github.com/keshon/beacon/internal/commands"
	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/notify"
	"github.com/keshon/beacon/internal/realtime"
	"github.com/keshon/beacon/internal/scheduler"
	"github.com/keshon/beacon/internal/store"
	"github.com/keshon/beacon/internal/sync"
	"github.com/keshon/beacon/internal/web"
)

func isCLISubcommand(s string) bool {
	return s == "monitor" || s == "state" || s == "events"
}

func loadConfig(st *store.Store, filePath string) *config.Config {
	var cfg config.Config
	ok, err := st.GetConfig(&cfg)
	if ok && err == nil {
		if cfg.Auth.Password != "" {
			cfg.RememberPlainPassword(cfg.Auth.Password)
		}
		if err := cfg.Auth.EnsureAuthHashed(); err != nil {
			log.Printf("auth hash: %v", err)
		}
		cfg.Normalize()
		_ = st.SetConfig(&cfg)
		return &cfg
	}
	if c, err := config.Load(filePath); err == nil {
		if c.Auth.Password != "" {
			c.RememberPlainPassword(c.Auth.Password)
		}
		_ = c.Auth.EnsureAuthHashed()
		c.Normalize()
		_ = st.SetConfig(c)
		return c
	}
	cfg = *config.Default()
	cfg.RememberPlainPassword(cfg.Auth.Password)
	_ = cfg.Auth.EnsureAuthHashed()
	cfg.Normalize()
	_ = st.SetConfig(&cfg)
	log.Printf("using default config")
	return &cfg
}

func main() {
	cfgPath := "config.json"
	if len(os.Args) > 1 && !isCLISubcommand(os.Args[1]) {
		cfgPath = os.Args[1]
	}

	dataDir := "data"
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, err := store.New(ctx, dataDir)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	if cli.RunCLI(st) {
		return
	}

	cfg := loadConfig(st, cfgPath)
	commands.RegisterAll(st)
	commands.RegisterConfigCommands(st, cfg)

	const alertQueueSize = 128
	alertQueue := make(chan func(), alertQueueSize)
	go func() {
		for fn := range alertQueue {
			fn()
		}
	}()

	sendAlerts := func(m *monitor.Monitor, state *monitor.MonitorState, result checks.CheckResult, status, message string, isRepeat bool) {
		receivers := notify.BuildReceivers(cfg, m)
		if len(receivers) == 0 {
			return
		}
		tplCtx := notify.NewTemplateContext(m, state, result, status, message)
		base := notify.Alert{
			MonitorName: m.Name,
			Status:      status,
			Message:     message,
			Time:        result.Time,
			Target:      m.Target,
			Type:        m.Type,
			StatusCode:  result.StatusCode,
			Latency:     result.Latency,
		}
		if state != nil {
			base.FailCount = state.FailCount
		}
		job := func() {
			for _, r := range receivers {
				if status == "down" && !notify.ShouldSendDown(r.Policy, isRepeat) {
					continue
				}
				alert := base
				alert.Body = notify.BuildAlertBody(r.Policy, status, tplCtx)
				if err := r.Notifier.Send(alert); err != nil {
					log.Printf("notify error [%s]: %v", r.Key, err)
				}
			}
		}
		select {
		case alertQueue <- job:
		default:
			log.Printf("notify queue full, dropping alert for %s", m.Name)
		}
	}

	engine := monitor.NewEngine(
		func(m *monitor.Monitor, state *monitor.MonitorState, result checks.CheckResult, isRepeat bool) {
			sendAlerts(m, state, result, "down", "Error: "+result.Error, isRepeat)
		},
		func(m *monitor.Monitor, state *monitor.MonitorState, result checks.CheckResult) {
			sendAlerts(m, state, result, "recovered", "Latency: "+result.Latency.String(), false)
		},
	)

	hub := realtime.NewHub()
	sch := scheduler.New(st, engine, cfg.Workers, cfg.DefaultIntervalDuration(), cfg, hub.BroadcastCheck)
	sch.Run(ctx)

	syncClient := sync.NewClient(st, cfg)
	go syncClient.Run(ctx)

	auth := web.NewAuth()
	srv := web.NewServer(st, auth, cfg, sch, "templates", "static", hub)
	httpServer := &http.Server{Addr: cfg.Listen, Handler: srv.Routes()}

	go func() {
		log.Printf("listening on http://localhost%s", cfg.Listen)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	log.Println("shutting down...")

	cancel()
	sch.Stop()
	close(alertQueue)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("http shutdown: %v", err)
	}

	log.Println("done")
}
