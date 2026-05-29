package scheduler

import (
	"context"
	"log"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/keshon/beacon/internal/checks"
	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/store"
)

type CheckJob struct {
	Monitor *monitor.Monitor
}

type Scheduler struct {
	store           *store.Store
	engine          *monitor.Engine
	workers         int
	defaultInterval time.Duration
	cfg             *config.Config
	onCheckRecorded func(store.CheckRecord, *monitor.MonitorState)
	jobs            chan CheckJob
	wg              sync.WaitGroup
	inFlight        map[string]struct{}
	inflMu          sync.Mutex
	droppedChecks   atomic.Uint64
}

func New(s *store.Store, engine *monitor.Engine, workers int, defaultInterval time.Duration, cfg *config.Config, onCheckRecorded func(store.CheckRecord, *monitor.MonitorState)) *Scheduler {
	if defaultInterval <= 0 {
		defaultInterval = 30 * time.Second
	}
	return &Scheduler{
		store:           s,
		engine:          engine,
		workers:         workers,
		defaultInterval: defaultInterval,
		cfg:             cfg,
		onCheckRecorded: onCheckRecorded,
		jobs:            make(chan CheckJob, 100),
		inFlight:        make(map[string]struct{}),
	}
}

func (sc *Scheduler) DroppedChecks() uint64 {
	return sc.droppedChecks.Load()
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
	var list []*monitor.Monitor
	own, err := sc.store.GetMonitors()
	if err != nil {
		return nil, err
	}
	for _, m := range own {
		list = append(list, m)
	}
	if sc.cfg == nil || !sc.cfg.Network.Enabled || sc.cfg.Network.NodeID == "" {
		return list, nil
	}
	deadTimeout := time.Duration(sc.cfg.Network.DeadTimeout) * time.Second
	peerData, err := sc.store.GetAllPeerData()
	if err != nil {
		return list, err
	}
	if len(peerData) == 0 {
		return list, nil
	}
	now := time.Now()
	var allNodes []string
	live := make(map[string]bool)
	allNodes = append(allNodes, sc.cfg.Network.NodeID)
	live[sc.cfg.Network.NodeID] = true
	for nodeID, pd := range peerData {
		allNodes = append(allNodes, nodeID)
		if now.Sub(pd.LastSeen) < deadTimeout {
			live[nodeID] = true
		}
	}
	sort.Strings(allNodes)
	for _, pd := range peerData {
		if now.Sub(pd.LastSeen) >= deadTimeout {
			deadID := pd.NodeID
			adopter := nextLiveInRing(allNodes, deadID, live)
			if adopter == sc.cfg.Network.NodeID {
				for _, m := range pd.Monitors {
					if !m.Enabled {
						continue
					}
					if err := monitor.ValidateTarget(m.Type, m.Target); err != nil {
						log.Printf("[scheduler] skip invalid peer monitor %s: %v", m.Name, err)
						continue
					}
					if st, ok := pd.State[m.ID]; ok && st != nil {
						local, _ := sc.store.GetState(m.ID)
						if local == nil {
							_ = sc.store.SetState(st)
						}
					}
					list = append(list, m)
				}
			}
		}
	}
	return list, nil
}

func nextLiveInRing(sorted []string, after string, live map[string]bool) string {
	for i, id := range sorted {
		if id == after {
			for j := 1; j < len(sorted); j++ {
				idx := (i + j) % len(sorted)
				if live[sorted[idx]] {
					return sorted[idx]
				}
			}
			return ""
		}
	}
	return ""
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
					jitter := time.Duration(rand.Intn(3000)) * time.Millisecond
					nextCheck = st.LastCheck.Add(interval).Add(jitter)
				} else {
					nextCheck = now
				}
				if nextCheck.Before(now) || nextCheck.Equal(now) {
					sc.inflMu.Lock()
					if _, ok := sc.inFlight[m.ID]; ok {
						sc.inflMu.Unlock()
						continue
					}
					sc.inFlight[m.ID] = struct{}{}
					sc.inflMu.Unlock()
					select {
					case sc.jobs <- CheckJob{Monitor: m}:
					case <-ctx.Done():
						sc.inflMu.Lock()
						delete(sc.inFlight, m.ID)
						sc.inflMu.Unlock()
						return
					default:
						sc.droppedChecks.Add(1)
						sc.inflMu.Lock()
						delete(sc.inFlight, m.ID)
						sc.inflMu.Unlock()
						log.Printf("[scheduler] job queue full, skipping %s (%s)", m.Name, m.ID)
					}
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
	m := job.Monitor
	defer func() {
		sc.inflMu.Lock()
		delete(sc.inFlight, m.ID)
		sc.inflMu.Unlock()
	}()
	timeout := m.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	var result checks.CheckResult
	switch m.Type {
	case "http":
		result = checks.HTTPCheck(ctx, m.Target, timeout)
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
	if err := sc.store.AppendCheckRecord(rec); err != nil {
		log.Printf("[scheduler] append event: %v", err)
	}
	st, err := sc.store.GetState(m.ID)
	if err != nil {
		log.Printf("[scheduler] read state: %v", err)
		return
	}
	if st == nil {
		st = &monitor.MonitorState{MonitorID: m.ID, Status: monitor.StatusUnknown}
	}
	sc.engine.Process(result, st, m)
	if err := sc.store.SetState(st); err != nil {
		log.Printf("[scheduler] write state: %v", err)
		return
	}

	if sc.onCheckRecorded != nil {
		sc.onCheckRecorded(rec, st)
	}

	log.Printf("[%s] monitor=%s status=%v latency=%v", result.Time.Format("15:04:05"), m.Name, result.Success, result.Latency)
}

func (sc *Scheduler) Stop() {
	close(sc.jobs)
	sc.wg.Wait()
}
