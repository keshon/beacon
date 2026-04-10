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
		cfg.Normalize()
		return &cfg
	}
	if c, err := config.Load(filePath); err == nil {
		c.Normalize()
		st.SetConfig(c)
		return c
	}
	cfg = *config.Default()
	cfg.Normalize()
	st.SetConfig(&cfg)
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

	sendAlert := func(alert notify.Alert, m *monitor.Monitor) {
		for _, n := range notify.BuildNotifiers(cfg, m) {
			if err := n.Send(alert); err != nil {
				log.Printf("notify error: %v", err)
			}
		}
	}

	engine := monitor.NewEngine(
		func(m *monitor.Monitor, state *monitor.MonitorState, result checks.CheckResult) {
			sendAlert(notify.Alert{
				MonitorName: m.Name,
				Status:      "down",
				Message:     "Error: " + result.Error,
				Time:        result.Time,
			}, m)
		},
		func(m *monitor.Monitor, state *monitor.MonitorState, result checks.CheckResult) {
			sendAlert(notify.Alert{
				MonitorName: m.Name,
				Status:      "recovered",
				Message:     "Latency: " + result.Latency.String(),
				Time:        result.Time,
			}, m)
		},
	)

	sch := scheduler.New(st, engine, cfg.Workers, cfg.DefaultIntervalDuration(), cfg)
	sch.Run(ctx)

	syncClient := sync.NewClient(st, cfg)
	go syncClient.Run(ctx)

	auth := web.NewAuth()
	srv := web.NewServer(st, auth, cfg, "templates", "static")
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

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("http shutdown: %v", err)
	}

	log.Println("done")
}
