package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/keshon/beacon/internal/monitor/checks"
	"github.com/keshon/beacon/internal/cluster"
	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/notify"
	"github.com/keshon/beacon/internal/monitor/scheduler"
	"github.com/keshon/beacon/internal/storage"
	"github.com/keshon/beacon/internal/server"
)

const (
	alertQueueSize      = 256
	alertEnqueueTimeout = 30 * time.Second
	alertDedupWindow    = 45 * time.Second
)

func loadConfig(st *storage.Store, filePath string) *config.Config {
	var cfg config.Config
	ok, err := st.GetConfig(&cfg)
	if ok && err == nil {
		if cfg.Auth.Password != "" {
			_ = cfg.Auth.SetPassword(cfg.Auth.Password)
		}
		if err := cfg.Auth.EnsureHashed(); err != nil {
			log.Printf("auth hash: %v", err)
		}
		cfg.Normalize()
		_ = st.SetConfig(&cfg)
		return &cfg
	}
	if c, err := config.Load(filePath); err == nil {
		if c.Auth.Password != "" {
			_ = c.Auth.SetPassword(c.Auth.Password)
		}
		_ = c.Auth.EnsureHashed()
		c.Normalize()
		_ = st.SetConfig(c)
		return c
	}
	cfg = *config.Default()
	_ = cfg.Auth.SetPassword(cfg.Auth.Password)
	_ = cfg.Auth.EnsureHashed()
	cfg.Normalize()
	_ = st.SetConfig(&cfg)
	log.Printf("using default config")
	return &cfg
}

func main() {
	cfgPath := "config.json"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}

	dataDir := "data"
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, err := storage.New(ctx, dataDir)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	serverLock, err := storage.AcquireDirLock(dataDir)
	if err != nil {
		log.Fatal(err)
	}
	defer serverLock.Release()

	cfg := loadConfig(st, cfgPath)

	alertQueue := make(chan func(), alertQueueSize)
	var alertWG sync.WaitGroup
	emailGuard := notify.NewEmailSendGuard()
	alertDedup := notify.NewAlertDedup()
	alertWG.Add(1)
	go func() {
		defer alertWG.Done()
		for fn := range alertQueue {
			fn()
		}
	}()

	sendAlerts := func(m *monitor.Monitor, state *monitor.MonitorState, result checks.CheckResult, status, message string, isRepeat bool) {
		if m == nil {
			return
		}
		if mon, err := st.GetMonitor(m.ID); err == nil {
			if mon == nil {
				// Adopted peer monitor — not in local store; allow alert.
			} else if !mon.Enabled {
				return
			}
		}
		if status == "down" && isRepeat && !alertDedup.Allow(m.ID, status, alertDedupWindow) {
			return
		}
		if status == "recovered" && !alertDedup.Allow(m.ID, status, alertDedupWindow) {
			return
		}

		receivers := notify.BuildReceivers(cfg, m)
		if len(receivers) == 0 {
			return
		}
		tplCtx := notify.WithNodeFromConfig(
			notify.NewTemplateContext(m, state, result, status, message),
			cfg,
		)
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
			for i, r := range receivers {
				if r.Channel == notify.ChannelEmail {
					if !emailGuard.Allow(m.ID, r.Key) {
						log.Printf("email cooldown skip [%s]", r.Key)
						continue
					}
				}
				if status == "down" && !notify.ShouldSendDown(r.Policy, isRepeat, r.Channel) {
					continue
				}
				alert := base
				alert.Body = notify.BuildAlertBody(r.Policy, status, tplCtx)
				if err := r.Notifier.Send(alert); err != nil {
					log.Printf("notify error [%s]: %v", r.Key, err)
				} else {
					if r.Channel == notify.ChannelEmail {
						emailGuard.RecordSuccess(m.ID, r.Key)
					}
					if i+1 < len(receivers) {
						time.Sleep(250 * time.Millisecond)
					}
				}
			}
		}
		select {
		case alertQueue <- job:
		case <-time.After(alertEnqueueTimeout):
			log.Printf("notify queue full after %s, dropping alert for %s", alertEnqueueTimeout, m.Name)
		}
	}

	evaluator := monitor.NewStatusEvaluator(
		func(m *monitor.Monitor, state *monitor.MonitorState, result checks.CheckResult, isRepeat bool) {
			sendAlerts(m, state, result, "down", "Error: "+result.Error, isRepeat)
		},
		func(m *monitor.Monitor, state *monitor.MonitorState, result checks.CheckResult) {
			sendAlerts(m, state, result, "recovered", "Latency: "+result.Latency.String(), false)
		},
	)

	streamHub := server.NewCheckStreamHub()

	clusterRT := cluster.New(st, cfg)
	var clusterWG sync.WaitGroup
	clusterWG.Add(1)
	go func() {
		defer clusterWG.Done()
		clusterRT.Run(ctx)
	}()

	src := scheduler.MonitorSource(clusterRT)

	sch := scheduler.New(st, src, evaluator, cfg.Workers, cfg.DefaultIntervalDuration(), cfg, streamHub.BroadcastCheck)
	sch.Run(ctx)
	notifyStartupDown(sch, sendAlerts)

	auth := server.NewAuth()
	srv := server.NewServer(st, auth, cfg, sch, clusterRT, "templates", "static", streamHub)
	httpServer := &http.Server{Addr: cfg.Listen, Handler: srv.Routes()}

	go func() {
		log.Printf("listening on http://localhost%s", cfg.Listen)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	log.Println("shutting down...")

	sch.Stop()
	cancel()
	clusterWG.Wait()

	close(alertQueue)
	alertWG.Wait()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("http shutdown: %v", err)
	}

	log.Println("done")
}

func notifyStartupDown(sch *scheduler.Scheduler, sendAlerts func(*monitor.Monitor, *monitor.MonitorState, checks.CheckResult, string, string, bool)) {
	monitors, states, err := sch.StartupDownMonitors()
	if err != nil {
		log.Printf("startup down sweep: %v", err)
		return
	}
	for _, m := range monitors {
		st := states[m.ID]
		if st == nil {
			continue
		}
		result := checks.CheckResult{
			MonitorID: m.ID,
			Success:   false,
			Error:     "still down after restart",
			Time:      time.Now(),
		}
		sendAlerts(m, st, result, "down", "Monitor still DOWN after Beacon restarted", false)
	}
	if len(monitors) > 0 {
		log.Printf("startup: notified %d monitor(s) still down", len(monitors))
	}
}
