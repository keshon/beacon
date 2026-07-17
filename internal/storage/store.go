package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/keshon/beacon/internal/monitor"
)

var ErrMonitorNotFound = errors.New("monitor not found")

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
	LastSeen   time.Time                        `json:"last_seen"`
	LastExport time.Time                        `json:"last_export,omitempty"`
	LastError  string                           `json:"last_error,omitempty"`
	SyncWarnings []string                       `json:"sync_warnings,omitempty"`
}

// Store keeps all data in memory under one lock and flushes each mutation to
// its JSON file via flushJSON — the single persistence path. Reads hand out
// copies, never pointers into the in-memory maps.
type Store struct {
	mu sync.RWMutex

	monitors map[string]*monitor.Monitor
	state    map[string]*monitor.MonitorState
	events   []CheckRecord
	config   json.RawMessage
	peers    map[string]*PeerData

	dataDir      string
	monitorsPath string
	statePath    string
	eventsPath   string
	configPath   string
	peerPath     string

	uptimeIdx map[string][]CheckRecord // per-monitor ring buffer, oldest first
}

const uptimeIndexLimit = 500

func New(dataDir string) (*Store, error) {
	s := &Store{
		monitors:     make(map[string]*monitor.Monitor),
		state:        make(map[string]*monitor.MonitorState),
		peers:        make(map[string]*PeerData),
		dataDir:      dataDir,
		monitorsPath: filepath.Join(dataDir, "monitors.json"),
		statePath:    filepath.Join(dataDir, "state.json"),
		eventsPath:   filepath.Join(dataDir, "events.json"),
		configPath:   filepath.Join(dataDir, "config.json"),
		peerPath:     filepath.Join(dataDir, "peer_data.json"),
		uptimeIdx:    make(map[string][]CheckRecord),
	}
	if err := loadWrapped(s.monitorsPath, keyMonitors, &s.monitors); err != nil {
		return nil, err
	}
	if err := loadWrapped(s.statePath, keyState, &s.state); err != nil {
		return nil, err
	}
	if err := loadWrapped(s.eventsPath, keyEvents, &s.events); err != nil {
		return nil, err
	}
	if err := loadWrapped(s.configPath, keyConfig, &s.config); err != nil {
		return nil, err
	}
	if err := loadWrapped(s.peerPath, keyPeerData, &s.peers); err != nil {
		return nil, err
	}
	if s.monitors == nil {
		s.monitors = make(map[string]*monitor.Monitor)
	}
	if s.state == nil {
		s.state = make(map[string]*monitor.MonitorState)
	}
	if s.peers == nil {
		s.peers = make(map[string]*PeerData)
	}
	for _, rec := range s.events {
		s.indexUptimeRecordLocked(rec)
	}
	return s, nil
}

// loadWrapped reads {"<key>": <value>} from path into dest. Missing file is fine.
func loadWrapped(path, key string, dest any) error {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %q: %w", path, err)
	}
	if len(raw) == 0 {
		return nil
	}
	var wrap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return fmt.Errorf("parse %q: %w", path, err)
	}
	v, ok := wrap[key]
	if !ok || len(v) == 0 || string(v) == "null" {
		return nil
	}
	if err := json.Unmarshal(v, dest); err != nil {
		return fmt.Errorf("parse %q key %q: %w", path, key, err)
	}
	return nil
}

// clone deep-copies v via a JSON round-trip (same semantics reads always had).
func clone[T any](v T) T {
	raw, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		return v
	}
	return out
}

func (s *Store) indexUptimeRecordLocked(rec CheckRecord) {
	buf := s.uptimeIdx[rec.MonitorID]
	buf = append(buf, rec)
	if len(buf) > uptimeIndexLimit {
		buf = buf[len(buf)-uptimeIndexLimit:]
	}
	s.uptimeIdx[rec.MonitorID] = buf
}

// Close is a no-op: every mutation is flushed synchronously.
func (s *Store) Close() error {
	return nil
}

func (s *Store) GetMonitors() ([]*monitor.Monitor, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.monitors) == 0 {
		return nil, nil
	}
	list := make([]*monitor.Monitor, 0, len(s.monitors))
	for _, v := range s.monitors {
		list = append(list, clone(v))
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
	m := s.monitors[id]
	if m == nil {
		return nil, nil
	}
	return clone(m), nil
}

func (s *Store) SetMonitor(mon *monitor.Monitor) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.monitors[mon.ID] = clone(mon)
	return flushJSON(s.monitorsPath, map[string]any{keyMonitors: s.monitors})
}

func (s *Store) DeleteMonitor(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.monitors[id] == nil {
		return ErrMonitorNotFound
	}
	delete(s.monitors, id)
	delete(s.state, id)
	filtered := s.events[:0]
	for _, rec := range s.events {
		if rec.MonitorID != id {
			filtered = append(filtered, rec)
		}
	}
	s.events = filtered
	delete(s.uptimeIdx, id)
	if err := flushJSON(s.monitorsPath, map[string]any{keyMonitors: s.monitors}); err != nil {
		return err
	}
	if err := flushJSON(s.statePath, map[string]any{keyState: s.state}); err != nil {
		return err
	}
	return flushJSON(s.eventsPath, map[string]any{keyEvents: s.events})
}

func (s *Store) GetState(monitorID string) (*monitor.MonitorState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := s.state[monitorID]
	if st == nil {
		return nil, nil
	}
	cp := *st
	return &cp, nil
}

func (s *Store) GetAllState() (map[string]*monitor.MonitorState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.state) == 0 {
		return nil, nil
	}
	out := make(map[string]*monitor.MonitorState, len(s.state))
	for id, st := range s.state {
		cp := *st
		out[id] = &cp
	}
	return out, nil
}

func (s *Store) SetState(st *monitor.MonitorState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *st
	s.state[st.MonitorID] = &cp
	return flushJSON(s.statePath, map[string]any{keyState: s.state})
}

func (s *Store) AppendCheckRecord(rec CheckRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.appendEventLocked(rec)
	return flushJSON(s.eventsPath, map[string]any{keyEvents: s.events})
}

func (s *Store) appendEventLocked(rec CheckRecord) {
	s.events = append(s.events, rec)
	if len(s.events) > 10000 {
		s.events = s.events[len(s.events)-10000:]
	}
	s.indexUptimeRecordLocked(rec)
}

func (s *Store) GetCheckRecords(limit int) ([]CheckRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.events) == 0 {
		return nil, nil
	}
	if limit <= 0 || limit > len(s.events) {
		limit = len(s.events)
	}
	start := len(s.events) - limit
	out := make([]CheckRecord, limit)
	copy(out, s.events[start:])
	return out, nil
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
	return s.uptimeSamplesLocked(monitorID, limit), nil
}

func (s *Store) uptimeSamplesLocked(monitorID string, limit int) []CheckRecord {
	buf := s.uptimeIdx[monitorID]
	if len(buf) == 0 {
		return nil
	}
	if limit > len(buf) {
		limit = len(buf)
	}
	start := len(buf) - limit
	out := make([]CheckRecord, limit)
	copy(out, buf[start:])
	return out
}

// GetUptimeSamplesBatch returns the last limit check outcomes for each monitor ID, oldest first.
func (s *Store) GetUptimeSamplesBatch(monitorIDs []string, limit int) (map[string][]CheckRecord, error) {
	if len(monitorIDs) == 0 {
		return map[string][]CheckRecord{}, nil
	}
	if limit <= 0 {
		limit = 120
	}
	const maxLimit = 500
	if limit > maxLimit {
		limit = maxLimit
	}

	want := make(map[string]struct{}, len(monitorIDs))
	for _, id := range monitorIDs {
		if id == "" {
			continue
		}
		want[id] = struct{}{}
	}
	if len(want) == 0 {
		return map[string][]CheckRecord{}, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make(map[string][]CheckRecord, len(want))
	for id := range want {
		out[id] = s.uptimeSamplesLocked(id, limit)
	}
	return out, nil
}

func (s *Store) GetConfig(dest any) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.config) == 0 {
		return false, nil
	}
	if err := json.Unmarshal(s.config, dest); err != nil {
		return false, fmt.Errorf("read config: %w", err)
	}
	return true, nil
}

func (s *Store) SetConfig(cfg any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	s.config = raw
	return flushJSON(s.configPath, map[string]any{keyConfig: json.RawMessage(raw)})
}

func (s *Store) GetPeerData(nodeID string) (*PeerData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pd := s.peers[nodeID]
	if pd == nil {
		return nil, nil
	}
	return clone(pd), nil
}

func (s *Store) SetPeerData(data *PeerData) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.peers[data.NodeID] = clone(data)
	return flushJSON(s.peerPath, map[string]any{keyPeerData: s.peers})
}

// SetPeerDataError records a sync error for the peer URL without replacing cached data.
func (s *Store) SetPeerDataError(peerURL, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	normalized := strings.TrimSuffix(peerURL, "/")
	for _, pd := range s.peers {
		if strings.TrimSuffix(pd.PeerURL, "/") == normalized {
			pd.LastError = errMsg
			return flushJSON(s.peerPath, map[string]any{keyPeerData: s.peers})
		}
	}
	return nil
}

// PrunePeerData removes cache entries whose peer URL is not in configuredPeers.
func (s *Store) PrunePeerData(configuredPeers []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.peers) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(configuredPeers))
	for _, u := range configuredPeers {
		u = strings.TrimSuffix(strings.TrimSpace(u), "/")
		if u != "" {
			allowed[u] = struct{}{}
		}
	}
	changed := false
	for nodeID, pd := range s.peers {
		key := strings.TrimSuffix(pd.PeerURL, "/")
		if key == "" {
			key = nodeID
		}
		if _, ok := allowed[key]; !ok {
			delete(s.peers, nodeID)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return flushJSON(s.peerPath, map[string]any{keyPeerData: s.peers})
}

func (s *Store) GetAllPeerData() (map[string]*PeerData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.peers) == 0 {
		return nil, nil
	}
	return clone(s.peers), nil
}

// Ping verifies the data directory is still accessible.
func (s *Store) Ping() error {
	if _, err := os.Stat(s.dataDir); err != nil {
		return fmt.Errorf("data dir: %w", err)
	}
	return nil
}

// flushJSON durably writes v to path (write temp + fsync + rename).
func flushJSON(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %q: %w", path, err)
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("write %q: %w", tmp, err)
	}
	if _, err := f.Write(raw); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("write %q: %w", tmp, err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("sync %q: %w", tmp, err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close %q: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename %q: %w", path, err)
	}
	return nil
}

// ExportSnapshot is a consistent monitors+state read for sync export.
type ExportSnapshot struct {
	Monitors []*monitor.Monitor
	State    map[string]*monitor.MonitorState
}

// GetExportSnapshot returns monitors and state under a single read lock.
func (s *Store) GetExportSnapshot() (ExportSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var snap ExportSnapshot
	if len(s.monitors) > 0 {
		snap.Monitors = make([]*monitor.Monitor, 0, len(s.monitors))
		for _, v := range s.monitors {
			snap.Monitors = append(snap.Monitors, clone(v))
		}
		sortMonitorsByName(snap.Monitors)
	}
	snap.State = make(map[string]*monitor.MonitorState)
	for id, st := range s.state {
		// Drop state entries with no matching local monitor.
		if s.monitors[id] == nil {
			continue
		}
		cp := *st
		snap.State[id] = &cp
	}
	return snap, nil
}

// MonitorExists reports whether a monitor ID is defined locally.
func (s *Store) MonitorExists(id string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.monitors[id]
	return exists, nil
}

// RecordCheckResult appends an event and updates state atomically, then flushes both files.
// When requireLocalMonitor is true, the monitor must exist in the local monitors map.
func (s *Store) RecordCheckResult(rec CheckRecord, st *monitor.MonitorState, requireLocalMonitor bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if requireLocalMonitor && s.monitors[rec.MonitorID] == nil {
		return ErrMonitorNotFound
	}

	s.appendEventLocked(rec)
	cp := *st
	s.state[st.MonitorID] = &cp

	if err := flushJSON(s.eventsPath, map[string]any{keyEvents: s.events}); err != nil {
		return err
	}
	return flushJSON(s.statePath, map[string]any{keyState: s.state})
}

// UpdateMonitor applies fn under the store write lock (read-modify-write).
func (s *Store) UpdateMonitor(id string, fn func(*monitor.Monitor) error) (*monitor.Monitor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	mon := s.monitors[id]
	if mon == nil {
		return nil, ErrMonitorNotFound
	}
	work := clone(mon)
	if err := fn(work); err != nil {
		return nil, err
	}
	s.monitors[id] = work
	if err := flushJSON(s.monitorsPath, map[string]any{keyMonitors: s.monitors}); err != nil {
		return nil, err
	}
	return clone(work), nil
}
