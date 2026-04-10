package store

import (
	"context"
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

type Event struct {
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
	eventsMu   sync.Mutex
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

func (s *Store) GetMonitors() []*monitor.Monitor {
	var m map[string]*monitor.Monitor
	ok, _ := s.monitorsDS.Get(keyMonitors, &m)
	if !ok || m == nil {
		return nil
	}
	list := make([]*monitor.Monitor, 0, len(m))
	for _, v := range m {
		list = append(list, v)
	}
	sortMonitorsByName(list)
	return list
}

func sortMonitorsByName(list []*monitor.Monitor) {
	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})
}

func (s *Store) GetMonitor(id string) *monitor.Monitor {
	var m map[string]*monitor.Monitor
	ok, _ := s.monitorsDS.Get(keyMonitors, &m)
	if !ok || m == nil {
		return nil
	}
	return m[id]
}

func (s *Store) SetMonitor(mon *monitor.Monitor) error {
	var m map[string]*monitor.Monitor
	ok, _ := s.monitorsDS.Get(keyMonitors, &m)
	if !ok || m == nil {
		m = make(map[string]*monitor.Monitor)
	}
	m[mon.ID] = mon
	return s.monitorsDS.Set(keyMonitors, m)
}

func (s *Store) DeleteMonitor(id string) error {
	var m map[string]*monitor.Monitor
	ok, _ := s.monitorsDS.Get(keyMonitors, &m)
	if !ok || m == nil {
		return nil
	}
	delete(m, id)
	if err := s.monitorsDS.Set(keyMonitors, m); err != nil {
		return err
	}
	// Remove state for deleted monitor
	var st map[string]*monitor.MonitorState
	ok, _ = s.stateDS.Get(keyState, &st)
	if ok && st != nil {
		delete(st, id)
		return s.stateDS.Set(keyState, st)
	}
	return nil
}

func (s *Store) GetState(monitorID string) *monitor.MonitorState {
	var st map[string]*monitor.MonitorState
	ok, _ := s.stateDS.Get(keyState, &st)
	if !ok || st == nil {
		return nil
	}
	return st[monitorID]
}

func (s *Store) GetAllState() map[string]*monitor.MonitorState {
	var st map[string]*monitor.MonitorState
	ok, _ := s.stateDS.Get(keyState, &st)
	if !ok || st == nil {
		return nil
	}
	return st
}

func (s *Store) SetState(st *monitor.MonitorState) error {
	var state map[string]*monitor.MonitorState
	ok, _ := s.stateDS.Get(keyState, &state)
	if !ok || state == nil {
		state = make(map[string]*monitor.MonitorState)
	}
	state[st.MonitorID] = st
	return s.stateDS.Set(keyState, state)
}

func (s *Store) AppendEvent(ev Event) error {
	s.eventsMu.Lock()
	defer s.eventsMu.Unlock()
	var events []Event
	ok, _ := s.eventsDS.Get(keyEvents, &events)
	if !ok || events == nil {
		events = make([]Event, 0)
	}
	events = append(events, ev)
	if len(events) > 10000 {
		events = events[len(events)-10000:]
	}
	return s.eventsDS.Set(keyEvents, events)
}

func (s *Store) GetEvents(limit int) []Event {
	var events []Event
	ok, _ := s.eventsDS.Get(keyEvents, &events)
	if !ok || events == nil {
		return nil
	}
	if limit <= 0 || limit > len(events) {
		limit = len(events)
	}
	start := len(events) - limit
	if start < 0 {
		start = 0
	}
	return events[start:]
}

func (s *Store) GetConfig(dest any) (bool, error) {
	return s.configDS.Get(keyConfig, dest)
}

func (s *Store) SetConfig(cfg any) error {
	return s.configDS.Set(keyConfig, cfg)
}

func (s *Store) GetPeerData(nodeID string) *PeerData {
	var m map[string]*PeerData
	ok, _ := s.peerDS.Get(keyPeerData, &m)
	if !ok || m == nil {
		return nil
	}
	return m[nodeID]
}

func (s *Store) SetPeerData(data *PeerData) error {
	var m map[string]*PeerData
	ok, _ := s.peerDS.Get(keyPeerData, &m)
	if !ok || m == nil {
		m = make(map[string]*PeerData)
	}
	m[data.NodeID] = data
	return s.peerDS.Set(keyPeerData, m)
}

func (s *Store) GetAllPeerData() map[string]*PeerData {
	var m map[string]*PeerData
	ok, _ := s.peerDS.Get(keyPeerData, &m)
	if !ok || m == nil {
		return nil
	}
	return m
}
