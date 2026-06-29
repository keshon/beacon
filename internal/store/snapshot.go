package store

import (
	"fmt"

	"github.com/keshon/beacon/internal/monitor"
)

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
	var m map[string]*monitor.Monitor
	ok, err := s.monitorsDS.Get(keyMonitors, &m)
	if err != nil {
		return snap, fmt.Errorf("read monitors: %w", err)
	}
	if ok && m != nil {
		snap.Monitors = make([]*monitor.Monitor, 0, len(m))
		for _, v := range m {
			snap.Monitors = append(snap.Monitors, v)
		}
		sortMonitorsByName(snap.Monitors)
	}
	var st map[string]*monitor.MonitorState
	ok, err = s.stateDS.Get(keyState, &st)
	if err != nil {
		return snap, fmt.Errorf("read state: %w", err)
	}
	if ok && st != nil {
		snap.State = st
	} else {
		snap.State = make(map[string]*monitor.MonitorState)
	}
	// Drop state entries with no matching local monitor.
	for id := range snap.State {
		if m == nil || m[id] == nil {
			delete(snap.State, id)
		}
	}
	return snap, nil
}

// MonitorExists reports whether a monitor ID is defined locally.
func (s *Store) MonitorExists(id string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var m map[string]*monitor.Monitor
	ok, err := s.monitorsDS.Get(keyMonitors, &m)
	if err != nil {
		return false, fmt.Errorf("read monitors: %w", err)
	}
	if !ok || m == nil {
		return false, nil
	}
	_, exists := m[id]
	return exists, nil
}

// RecordCheckResult appends an event and updates state atomically, then flushes both files.
// When requireLocalMonitor is true, the monitor must exist in the local monitors map.
func (s *Store) RecordCheckResult(rec CheckRecord, st *monitor.MonitorState, requireLocalMonitor bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if requireLocalMonitor {
		var m map[string]*monitor.Monitor
		ok, err := s.monitorsDS.Get(keyMonitors, &m)
		if err != nil {
			return fmt.Errorf("read monitors: %w", err)
		}
		if !ok || m == nil || m[rec.MonitorID] == nil {
			return ErrMonitorNotFound
		}
	}

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
	if err := s.eventsDS.Set(keyEvents, events); err != nil {
		return err
	}

	var state map[string]*monitor.MonitorState
	ok, err = s.stateDS.Get(keyState, &state)
	if err != nil {
		return fmt.Errorf("read state: %w", err)
	}
	if !ok || state == nil {
		state = make(map[string]*monitor.MonitorState)
	}
	state[st.MonitorID] = st
	if err := s.stateDS.Set(keyState, state); err != nil {
		return err
	}

	if err := flushJSON(s.eventsPath, map[string]any{keyEvents: events}); err != nil {
		return err
	}
	return flushJSON(s.statePath, map[string]any{keyState: state})
}

// UpdateMonitor applies fn under the store write lock (read-modify-write).
func (s *Store) UpdateMonitor(id string, fn func(*monitor.Monitor) error) (*monitor.Monitor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var m map[string]*monitor.Monitor
	ok, err := s.monitorsDS.Get(keyMonitors, &m)
	if err != nil {
		return nil, fmt.Errorf("read monitors: %w", err)
	}
	if !ok || m == nil {
		return nil, ErrMonitorNotFound
	}
	mon := m[id]
	if mon == nil {
		return nil, ErrMonitorNotFound
	}
	if err := fn(mon); err != nil {
		return nil, err
	}
	if err := s.monitorsDS.Set(keyMonitors, m); err != nil {
		return nil, err
	}
	if err := flushJSON(s.monitorsPath, map[string]any{keyMonitors: m}); err != nil {
		return nil, err
	}
	return mon, nil
}
