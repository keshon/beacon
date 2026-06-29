package scheduler

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/keshon/beacon/internal/checks"
	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/network"
	"github.com/keshon/beacon/internal/notify"
	"github.com/keshon/beacon/internal/store"
)

const (
	jobQueueSize     = 100
	enqueueWait      = 5 * time.Second
	jitterMax        = 3 * time.Second
)

type CheckJob struct {
	MonitorID       string
	RequireLocalMon bool
}

type Scheduler struct {
	store           *store.Store
	evaluator       *monitor.StatusEvaluator
	workers         int
	defaultInterval time.Duration
	cfg             *config.Config
	onCheckRecorded func(store.CheckRecord, *monitor.MonitorState)
	jobs            chan CheckJob
	wg              sync.WaitGroup
	inFlight        map[string]struct{}
	inflMu          sync.Mutex
	droppedChecks   atomic.Uint64
	stopOnce        sync.Once
	stopping        atomic.Bool
}

func New(s *store.Store, evaluator *monitor.StatusEvaluator, workers int, defaultInterval time.Duration, cfg *config.Config, onCheckRecorded func(store.CheckRecord, *monitor.MonitorState)) *Scheduler {
	if workers <= 0 {
		workers = 10
	}
	if defaultInterval <= 0 {
		defaultInterval = 30 * time.Second
	}
	return &Scheduler{
		store:           s,
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

// ReloadConfig updates scheduler tuning from live config.
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

func (sc *Scheduler) getMonitorsToCheck() ([]*monitor.Monitor, error) {
	own, err := sc.store.GetMonitors()
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*monitor.Monitor, len(own))
	for _, m := range own {
		byID[m.ID] = m
	}

	if sc.cfg == nil || !sc.cfg.Network.Enabled || sc.cfg.Network.NodeID == "" {
		return own, nil
	}

	peerData, err := sc.store.GetAllPeerData()
	if err != nil {
		return own, err
	}
	now := time.Now()
	adopted := network.AdoptedMonitors(sc.cfg, peerData, now)
	for _, am := range adopted {
		if existing, ok := byID[am.Monitor.ID]; ok {
			log.Printf("[scheduler] monitor ID collision %s: keeping local definition over peer %s", am.Monitor.ID, am.OwnerNodeID)
			_ = existing
			continue
		}
		byID[am.Monitor.ID] = am.Monitor
		if st, ok := peerData[am.OwnerNodeID]; ok && st != nil {
			if peerSt, ok := st.State[am.Monitor.ID]; ok && peerSt != nil {
				local, _ := sc.store.GetState(am.Monitor.ID)
				merged := network.MergeMonitorState(local, peerSt)
				if merged != nil && (local == nil || merged.LastCheck.After(local.LastCheck)) {
					_ = sc.store.SetState(merged)
				}
			}
		}
	}

	list := make([]*monitor.Monitor, 0, len(byID))
	for _, m := range byID {
		list = append(list, m)
	}
	return list, nil
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
			monitors, err := sc.getMonitorsToCheck()
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

				job := CheckJob{MonitorID: m.ID, RequireLocalMon: true}
				if sc.cfg != nil && sc.cfg.Network.Enabled {
					if local, _ := sc.store.GetMonitor(m.ID); local == nil {
						job.RequireLocalMon = false
					}
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

	m, err := sc.resolveMonitor(job.MonitorID)
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

	rec := store.CheckRecord{
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
		if err == store.ErrMonitorNotFound {
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

// StartupDownMonitors returns monitors persisted in down state for restart notification.
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

func (sc *Scheduler) resolveMonitor(id string) (*monitor.Monitor, error) {
	m, err := sc.store.GetMonitor(id)
	if err != nil {
		return nil, err
	}
	if m != nil {
		return m, nil
	}
	if sc.cfg == nil || !sc.cfg.Network.Enabled {
		return nil, nil
	}
	peerData, err := sc.store.GetAllPeerData()
	if err != nil {
		return nil, err
	}
	for _, am := range network.AdoptedMonitors(sc.cfg, peerData, time.Now()) {
		if am.Monitor != nil && am.Monitor.ID == id {
			return am.Monitor, nil
		}
	}
	return nil, nil
}
