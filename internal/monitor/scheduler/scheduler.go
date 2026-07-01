package scheduler

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/keshon/beacon/internal/monitor/checks"
	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/notify"
	"github.com/keshon/beacon/internal/storage"
)

const (
	jobQueueSize = 100
	enqueueWait  = 5 * time.Second
	jitterMax    = 3 * time.Second
)

type CheckJob struct {
	MonitorID       string
	RequireLocalMon bool
}

type Scheduler struct {
	store           *storage.Store
	source          MonitorSource
	evaluator       *monitor.StatusEvaluator
	workers         int
	defaultInterval time.Duration
	cfg             *config.Config
	onCheckRecorded func(storage.CheckRecord, *monitor.MonitorState)
	jobs            chan CheckJob
	wg              sync.WaitGroup
	inFlight        map[string]struct{}
	inflMu          sync.Mutex
	droppedChecks   atomic.Uint64
	stopOnce        sync.Once
	stopping        atomic.Bool
}

func New(s *storage.Store, source MonitorSource, evaluator *monitor.StatusEvaluator, workers int, defaultInterval time.Duration, cfg *config.Config, onCheckRecorded func(storage.CheckRecord, *monitor.MonitorState)) *Scheduler {
	if source == nil {
		source = LocalSource{Store: s}
	}
	if workers <= 0 {
		workers = 10
	}
	if defaultInterval <= 0 {
		defaultInterval = 30 * time.Second
	}
	return &Scheduler{
		store:           s,
		source:          source,
		evaluator:       evaluator,
		workers:         workers,
		defaultInterval: defaultInterval,
		cfg:             cfg,
		onCheckRecorded: onCheckRecorded,
		jobs:            make(chan CheckJob, jobQueueSize),
		inFlight:        make(map[string]struct{}),
	}
}

func (sc *Scheduler) DroppedChecks() uint64 {
	return sc.droppedChecks.Load()
}

func (sc *Scheduler) ReloadConfig(cfg *config.Config) {
	if cfg == nil {
		return
	}
	sc.cfg = cfg
	if cfg.Workers > 0 {
		sc.workers = cfg.Workers
	}
	if d := cfg.DefaultIntervalDuration(); d > 0 {
		sc.defaultInterval = d
	}
}

func (sc *Scheduler) Run(ctx context.Context) {
	for i := 0; i < sc.workers; i++ {
		sc.wg.Add(1)
		go sc.worker(ctx)
	}
	sc.wg.Add(1)
	go sc.loop(ctx)
}

func (sc *Scheduler) loop(ctx context.Context) {
	defer sc.wg.Done()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			monitors, err := sc.source.List(ctx)
			if err != nil {
				log.Printf("[scheduler] list monitors: %v", err)
				continue
			}
			for _, m := range monitors {
				if !m.Enabled {
					continue
				}
				st, err := sc.store.GetState(m.ID)
				if err != nil {
					log.Printf("[scheduler] read state %s: %v", m.ID, err)
					continue
				}
				var nextCheck time.Time
				if st != nil && !st.LastCheck.IsZero() {
					interval := m.Interval
					if interval <= 0 {
						interval = sc.defaultInterval
					}
					minInterval := notify.RecommendedMinInterval(sc.cfg, m)
					if minInterval > interval {
						interval = minInterval
					}
					jitter := stableJitter(m.ID)
					nextCheck = st.LastCheck.Add(interval).Add(jitter)
				} else {
					nextCheck = now
				}
				if nextCheck.After(now) {
					continue
				}
				sc.inflMu.Lock()
				if _, ok := sc.inFlight[m.ID]; ok {
					sc.inflMu.Unlock()
					continue
				}
				sc.inFlight[m.ID] = struct{}{}
				sc.inflMu.Unlock()

				job := CheckJob{
					MonitorID:       m.ID,
					RequireLocalMon: sc.source.RequireLocalMonitor(m.ID),
				}
				if sc.stopping.Load() {
					sc.inflMu.Lock()
					delete(sc.inFlight, m.ID)
					sc.inflMu.Unlock()
					continue
				}
				select {
				case sc.jobs <- job:
				case <-ctx.Done():
					sc.inflMu.Lock()
					delete(sc.inFlight, m.ID)
					sc.inflMu.Unlock()
					return
				case <-time.After(enqueueWait):
					sc.droppedChecks.Add(1)
					sc.inflMu.Lock()
					delete(sc.inFlight, m.ID)
					sc.inflMu.Unlock()
					log.Printf("[scheduler] job queue full after %s, skipping %s (%s)", enqueueWait, m.Name, m.ID)
				}
			}
		}
	}
}

func (sc *Scheduler) worker(ctx context.Context) {
	defer sc.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-sc.jobs:
			if !ok {
				return
			}
			sc.runCheck(ctx, job)
		}
	}
}

func (sc *Scheduler) runCheck(ctx context.Context, job CheckJob) {
	defer func() {
		sc.inflMu.Lock()
		delete(sc.inFlight, job.MonitorID)
		sc.inflMu.Unlock()
	}()

	m, err := sc.source.Resolve(job.MonitorID)
	if err != nil {
		log.Printf("[scheduler] read monitor %s: %v", job.MonitorID, err)
		return
	}
	if m == nil || !m.Enabled {
		return
	}

	timeout := m.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	var result checks.CheckResult
	switch m.Type {
	case "http":
		result = checks.HTTPCheck(ctx, m.Target, timeout, m.HTTP)
	case "tcp":
		result = checks.TCPCheck(ctx, m.Target, timeout)
	default:
		result = checks.CheckResult{
			MonitorID: m.ID,
			Success:   false,
			Error:     "unknown check type: " + m.Type,
			Time:      time.Now(),
		}
	}
	result.MonitorID = m.ID

	rec := storage.CheckRecord{
		MonitorID: m.ID,
		Success:   result.Success,
		Time:      result.Time,
		Latency:   result.Latency,
		Error:     result.Error,
	}

	st, err := sc.store.GetState(m.ID)
	if err != nil {
		log.Printf("[scheduler] read state: %v", err)
		return
	}
	if st == nil {
		st = &monitor.MonitorState{MonitorID: m.ID, Status: monitor.StatusUnknown}
	}
	sc.evaluator.Process(result, st, m)

	if err := sc.store.RecordCheckResult(rec, st, job.RequireLocalMon); err != nil {
		if err == storage.ErrMonitorNotFound {
			return
		}
		log.Printf("[scheduler] persist check: %v", err)
		return
	}

	if sc.onCheckRecorded != nil {
		sc.onCheckRecorded(rec, st)
	}

	log.Printf("[%s] monitor=%s status=%v latency=%v", result.Time.Format("15:04:05"), m.Name, result.Success, result.Latency)
}

func (sc *Scheduler) Stop() {
	sc.stopOnce.Do(func() {
		sc.stopping.Store(true)
		close(sc.jobs)
	})
	sc.wg.Wait()
}

func (sc *Scheduler) StartupDownMonitors() ([]*monitor.Monitor, map[string]*monitor.MonitorState, error) {
	monitors, err := sc.store.GetMonitors()
	if err != nil {
		return nil, nil, err
	}
	state, err := sc.store.GetAllState()
	if err != nil {
		return nil, nil, err
	}
	var down []*monitor.Monitor
	downState := make(map[string]*monitor.MonitorState)
	for _, m := range monitors {
		if !m.Enabled {
			continue
		}
		st := state[m.ID]
		if st != nil && st.Status == monitor.StatusDown {
			down = append(down, m)
			downState[m.ID] = st
		}
	}
	return down, downState, nil
}

func stableJitter(monitorID string) time.Duration {
	var h uint32
	for i := 0; i < len(monitorID); i++ {
		h = h*31 + uint32(monitorID[i])
	}
	ms := h % uint32(jitterMax/time.Millisecond)
	return time.Duration(ms) * time.Millisecond
}
