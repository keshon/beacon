package scheduler

import (
	"context"
	"log"
	"math/rand"
	"sort"
	"sync"
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
	jobs            chan CheckJob
	done            chan struct{}
	wg              sync.WaitGroup
	inFlight        map[string]struct{}
	inflMu          sync.Mutex
}

func New(s *store.Store, engine *monitor.Engine, workers int, defaultInterval time.Duration, cfg *config.Config) *Scheduler {
	if defaultInterval <= 0 {
		defaultInterval = 30 * time.Second
	}
	return &Scheduler{
		store:           s,
		engine:          engine,
		workers:         workers,
		defaultInterval: defaultInterval,
		cfg:             cfg,
		jobs:            make(chan CheckJob, 100),
		done:            make(chan struct{}),
		inFlight:        make(map[string]struct{}),
	}
}

func (sc *Scheduler) Run(ctx context.Context) {
	// Start workers
	for i := 0; i < sc.workers; i++ {
		sc.wg.Add(1)
		go sc.worker(ctx)
	}

	// Main scheduling loop
	sc.wg.Add(1)
	go sc.loop(ctx)
}

func (sc *Scheduler) getMonitorsToCheck() []*monitor.Monitor {
	var list []*monitor.Monitor
	own := sc.store.GetMonitors()
	for _, m := range own {
		list = append(list, m)
	}
	if sc.cfg == nil || !sc.cfg.Network.Enabled || sc.cfg.Network.NodeID == "" {
		return list
	}
	deadTimeout := time.Duration(sc.cfg.Network.DeadTimeout) * time.Second
	peerData := sc.store.GetAllPeerData()
	if len(peerData) == 0 {
		return list
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
					if st, ok := pd.State[m.ID]; ok && st != nil && sc.store.GetState(m.ID) == nil {
						sc.store.SetState(st)
					}
					list = append(list, m)
				}
			}
		}
	}
	return list
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
			for _, m := range sc.getMonitorsToCheck() {
				if !m.Enabled {
					continue
				}
				st := sc.store.GetState(m.ID)
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
					default:
						sc.inflMu.Lock()
						delete(sc.inFlight, m.ID)
						sc.inflMu.Unlock()
						log.Printf("[scheduler] job queue full, skipping %s", m.Name)
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
		result = checks.HTTPCheck(m.Target, timeout)
	case "tcp":
		result = checks.TCPCheck(m.Target, timeout)
	default:
		result = checks.CheckResult{
			MonitorID: m.ID,
			Success:   false,
			Error:     "unknown check type: " + m.Type,
			Time:      time.Now(),
		}
	}
	result.MonitorID = m.ID

	// Record event
	sc.store.AppendEvent(store.Event{
		MonitorID: m.ID,
		Success:   result.Success,
		Time:      result.Time,
		Latency:   result.Latency,
		Error:     result.Error,
	})

	// Update state via engine
	st := sc.store.GetState(m.ID)
	if st == nil {
		st = &monitor.MonitorState{MonitorID: m.ID, Status: monitor.StatusUnknown}
	}
	sc.engine.Process(result, st, m)
	sc.store.SetState(st)

	log.Printf("[%s] monitor=%s status=%v latency=%v", result.Time.Format("15:04:05"), m.Name, result.Success, result.Latency)
}

func (sc *Scheduler) Stop() {
	close(sc.done)
	close(sc.jobs)
	sc.wg.Wait()
}
