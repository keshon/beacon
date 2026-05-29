package store

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/datastore"
)

const (
	keyMonitors = "monitors"
	keyState    = "state"
	keyEvents   = "events"
	keyConfig   = "config"
	keyPeerData = "peer_data"
)

// CheckRecord is one persisted outcome of a monitor probe (uptime history sample).
type CheckRecord struct {
	MonitorID string        `json:"monitor_id"`
	Success   bool          `json:"success"`
	Time      time.Time     `json:"time"`
	Latency   time.Duration `json:"latency"`
	Error     string        `json:"error"`
}

// PeerData holds synced data from a peer node.
type PeerData struct {
	NodeID    string                           `json:"node_id"`
	PeerURL   string                           `json:"peer_url,omitempty"`
	Monitors  []*monitor.Monitor               `json:"monitors"`
	State     map[string]*monitor.MonitorState `json:"state"`
	LastSeen  time.Time                        `json:"last_seen"`
	LastError string                           `json:"last_error,omitempty"`
}

type Store struct {
	monitorsDS *datastore.DataStore
	stateDS    *datastore.DataStore
	eventsDS   *datastore.DataStore
	configDS   *datastore.DataStore
	peerDS     *datastore.DataStore
	mu         sync.RWMutex
}

func New(ctx context.Context, dataDir string) (*Store, error) {
	monitorsDS, err := datastore.New(ctx, filepath.Join(dataDir, "monitors.json"))
	if err != nil {
		return nil, err
	}
	stateDS, err := datastore.New(ctx, filepath.Join(dataDir, "state.json"))
	if err != nil {
		monitorsDS.Close()
		return nil, err
	}
	eventsDS, err := datastore.New(ctx, filepath.Join(dataDir, "events.json"))
	if err != nil {
		monitorsDS.Close()
		stateDS.Close()
		return nil, err
	}
	configDS, err := datastore.New(ctx, filepath.Join(dataDir, "config.json"))
	if err != nil {
		monitorsDS.Close()
		stateDS.Close()
		eventsDS.Close()
		return nil, err
	}
	peerDS, err := datastore.New(ctx, filepath.Join(dataDir, "peer_data.json"))
	if err != nil {
		monitorsDS.Close()
		stateDS.Close()
		eventsDS.Close()
		configDS.Close()
		return nil, err
	}
	return &Store{
		monitorsDS: monitorsDS,
		stateDS:    stateDS,
		eventsDS:   eventsDS,
		configDS:   configDS,
		peerDS:     peerDS,
	}, nil
}

func (s *Store) Close() error {
	var errs []error
	if err := s.monitorsDS.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := s.stateDS.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := s.eventsDS.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := s.configDS.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := s.peerDS.Close(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func (s *Store) GetMonitors() ([]*monitor.Monitor, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var m map[string]*monitor.Monitor
	ok, err := s.monitorsDS.Get(keyMonitors, &m)
	if err != nil {
		return nil, fmt.Errorf("read monitors: %w", err)
	}
	if !ok || m == nil {
		return nil, nil
	}
	list := make([]*monitor.Monitor, 0, len(m))
	for _, v := range m {
		list = append(list, v)
	}
	sortMonitorsByName(list)
	return list, nil
}

func sortMonitorsByName(list []*monitor.Monitor) {
	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})
}

func (s *Store) GetMonitor(id string) (*monitor.Monitor, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var m map[string]*monitor.Monitor
	ok, err := s.monitorsDS.Get(keyMonitors, &m)
	if err != nil {
		return nil, fmt.Errorf("read monitors: %w", err)
	}
	if !ok || m == nil {
		return nil, nil
	}
	return m[id], nil
}

func (s *Store) SetMonitor(mon *monitor.Monitor) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var m map[string]*monitor.Monitor
	ok, err := s.monitorsDS.Get(keyMonitors, &m)
	if err != nil {
		return fmt.Errorf("read monitors: %w", err)
	}
	if !ok || m == nil {
		m = make(map[string]*monitor.Monitor)
	}
	m[mon.ID] = mon
	return s.monitorsDS.Set(keyMonitors, m)
}

func (s *Store) DeleteMonitor(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var m map[string]*monitor.Monitor
	ok, err := s.monitorsDS.Get(keyMonitors, &m)
	if err != nil {
		return fmt.Errorf("read monitors: %w", err)
	}
	if !ok || m == nil {
		return nil
	}
	delete(m, id)
	if err := s.monitorsDS.Set(keyMonitors, m); err != nil {
		return err
	}
	var st map[string]*monitor.MonitorState
	ok, err = s.stateDS.Get(keyState, &st)
	if err != nil {
		return fmt.Errorf("read state: %w", err)
	}
	if ok && st != nil {
		delete(st, id)
		return s.stateDS.Set(keyState, st)
	}
	return nil
}

func (s *Store) GetState(monitorID string) (*monitor.MonitorState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var st map[string]*monitor.MonitorState
	ok, err := s.stateDS.Get(keyState, &st)
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}
	if !ok || st == nil {
		return nil, nil
	}
	return st[monitorID], nil
}

func (s *Store) GetAllState() (map[string]*monitor.MonitorState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var st map[string]*monitor.MonitorState
	ok, err := s.stateDS.Get(keyState, &st)
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}
	if !ok || st == nil {
		return nil, nil
	}
	return st, nil
}

func (s *Store) SetState(st *monitor.MonitorState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var state map[string]*monitor.MonitorState
	ok, err := s.stateDS.Get(keyState, &state)
	if err != nil {
		return fmt.Errorf("read state: %w", err)
	}
	if !ok || state == nil {
		state = make(map[string]*monitor.MonitorState)
	}
	state[st.MonitorID] = st
	return s.stateDS.Set(keyState, state)
}

func (s *Store) AppendCheckRecord(rec CheckRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var events []CheckRecord
	ok, err := s.eventsDS.Get(keyEvents, &events)
	if err != nil {
		return fmt.Errorf("read events: %w", err)
	}
	if !ok || events == nil {
		events = make([]CheckRecord, 0)
	}
	events = append(events, rec)
	if len(events) > 10000 {
		events = events[len(events)-10000:]
	}
	return s.eventsDS.Set(keyEvents, events)
}

func (s *Store) GetCheckRecords(limit int) ([]CheckRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var events []CheckRecord
	ok, err := s.eventsDS.Get(keyEvents, &events)
	if err != nil {
		return nil, fmt.Errorf("read events: %w", err)
	}
	if !ok || events == nil {
		return nil, nil
	}
	if limit <= 0 || limit > len(events) {
		limit = len(events)
	}
	start := len(events) - limit
	if start < 0 {
		start = 0
	}
	return events[start:], nil
}

// GetUptimeSamples returns the last limit check outcomes for monitorID, oldest first.
func (s *Store) GetUptimeSamples(monitorID string, limit int) ([]CheckRecord, error) {
	if limit <= 0 {
		limit = 120
	}
	const maxLimit = 500
	if limit > maxLimit {
		limit = maxLimit
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var events []CheckRecord
	ok, err := s.eventsDS.Get(keyEvents, &events)
	if err != nil {
		return nil, fmt.Errorf("read events: %w", err)
	}
	if !ok || events == nil {
		return nil, nil
	}
	out := make([]CheckRecord, 0, limit)
	for i := len(events) - 1; i >= 0 && len(out) < limit; i-- {
		if events[i].MonitorID == monitorID {
			out = append(out, events[i])
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func (s *Store) GetConfig(dest any) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ok, err := s.configDS.Get(keyConfig, dest)
	if err != nil {
		return false, fmt.Errorf("read config: %w", err)
	}
	return ok, err
}

func (s *Store) SetConfig(cfg any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.configDS.Set(keyConfig, cfg)
}

func (s *Store) GetPeerData(nodeID string) (*PeerData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var m map[string]*PeerData
	ok, err := s.peerDS.Get(keyPeerData, &m)
	if err != nil {
		return nil, fmt.Errorf("read peer data: %w", err)
	}
	if !ok || m == nil {
		return nil, nil
	}
	return m[nodeID], nil
}

func (s *Store) SetPeerData(data *PeerData) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var m map[string]*PeerData
	ok, err := s.peerDS.Get(keyPeerData, &m)
	if err != nil {
		return fmt.Errorf("read peer data: %w", err)
	}
	if !ok || m == nil {
		m = make(map[string]*PeerData)
	}
	m[data.NodeID] = data
	return s.peerDS.Set(keyPeerData, m)
}

func (s *Store) GetAllPeerData() (map[string]*PeerData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var m map[string]*PeerData
	ok, err := s.peerDS.Get(keyPeerData, &m)
	if err != nil {
		return nil, fmt.Errorf("read peer data: %w", err)
	}
	if !ok || m == nil {
		return nil, nil
	}
	return m, nil
}

// Ping verifies all datastore backends are readable.
func (s *Store) Ping() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var dummy any
	if _, err := s.monitorsDS.Get(keyMonitors, &dummy); err != nil {
		return fmt.Errorf("monitors store: %w", err)
	}
	if _, err := s.stateDS.Get(keyState, &dummy); err != nil {
		return fmt.Errorf("state store: %w", err)
	}
	if _, err := s.eventsDS.Get(keyEvents, &dummy); err != nil {
		return fmt.Errorf("events store: %w", err)
	}
	if _, err := s.configDS.Get(keyConfig, &dummy); err != nil {
		return fmt.Errorf("config store: %w", err)
	}
	if _, err := s.peerDS.Get(keyPeerData, &dummy); err != nil {
		return fmt.Errorf("peer store: %w", err)
	}
	return nil
}
