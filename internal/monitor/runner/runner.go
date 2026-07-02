package runner

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/keshon/beacon/internal/monitor/checks"
	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/storage"
)
// AddInput is the shared input for creating a monitor (CLI, API, etc.).
type AddInput struct {
	Name           string
	Type           string
	Target         string
	IntervalSec    int
	TimeoutSec     int
	Retries        int
	HTTP           *checks.HTTPOptions
	NotifyOverride *monitor.NotifyOverride
}

// Add validates input, persists a new monitor, and returns the stored record.
func Add(st *storage.Store, in AddInput) (*monitor.Monitor, error) {
	name := strings.TrimSpace(in.Name)
	target := strings.TrimSpace(in.Target)
	if name == "" || target == "" {
		return nil, fmt.Errorf("name and target are required")
	}
	typ, err := monitor.NormalizeType(in.Type)
	if err != nil {
		return nil, err
	}
	if err := monitor.ValidateTarget(typ, target); err != nil {
		return nil, err
	}
	m := &monitor.Monitor{
		ID:       uuid.New().String(),
		Name:     name,
		Type:     typ,
		Target:   target,
		Interval: 0,
		Timeout:  10 * time.Second,
		Retries:  3,
		Enabled:  true,
	}
	if in.IntervalSec > 0 {
		m.Interval = time.Duration(in.IntervalSec) * time.Second
	}
	if in.TimeoutSec > 0 {
		m.Timeout = time.Duration(in.TimeoutSec) * time.Second
	}
	if in.Retries > 0 {
		m.Retries = in.Retries
	}
	if in.NotifyOverride != nil {
		m.NotifyOverride = monitor.SanitizeNotifyOverride(in.NotifyOverride)
	}
	if in.HTTP != nil {
		m.HTTP = in.HTTP
	}
	var cfg config.Config
	if ok, err := st.GetConfig(&cfg); err == nil && ok && cfg.Network.Enabled && cfg.Network.NodeID != "" {
		m.OwnerNodeID = cfg.Network.NodeID
	}
	if err := st.SetMonitor(m); err != nil {
		return nil, err
	}
	return m, nil
}

// UpdatePatch is a partial monitor update (nil fields are left unchanged).
type UpdatePatch struct {
	Enabled        *bool
	Name           *string
	Type           *string
	Target         *string
	IntervalSec    *int
	HTTP           *checks.HTTPOptions
	NotifyOverride *monitor.NotifyOverride
}

// Update applies patch to an existing monitor and returns the updated record.
func Update(st *storage.Store, id string, patch UpdatePatch) (*monitor.Monitor, error) {
	return st.UpdateMonitor(id, func(mon *monitor.Monitor) error {
		if patch.Enabled != nil {
			mon.Enabled = *patch.Enabled
		}
		if patch.Name != nil {
			mon.Name = *patch.Name
		}
		if patch.Type != nil {
			typ, err := monitor.NormalizeType(*patch.Type)
			if err != nil {
				return err
			}
			mon.Type = typ
		}
		if patch.Target != nil {
			mon.Target = strings.TrimSpace(*patch.Target)
		}
		if patch.Type != nil || patch.Target != nil {
			if err := monitor.ValidateTarget(mon.Type, mon.Target); err != nil {
				return err
			}
		}
		if patch.IntervalSec != nil {
			if *patch.IntervalSec > 0 {
				mon.Interval = time.Duration(*patch.IntervalSec) * time.Second
			} else {
				mon.Interval = 0
			}
		}
		if patch.HTTP != nil {
			mon.HTTP = monitor.MergeHTTPOptions(mon.HTTP, patch.HTTP)
		}
		if patch.NotifyOverride != nil {
			sanitized := monitor.SanitizeNotifyOverride(patch.NotifyOverride)
			if sanitized != nil {
				mon.NotifyOverride = sanitized
			}
		}
		return nil
	})
}

// Delete removes a monitor by ID.
func Delete(st *storage.Store, id string) error {
	return st.DeleteMonitor(id)
}

// List returns all monitors from the store.
func List(st *storage.Store) ([]*monitor.Monitor, error) {
	return st.GetMonitors()
}

// ListRedacted returns monitors with secrets omitted (API-safe).
func ListRedacted(st *storage.Store) ([]*monitor.Monitor, error) {
	list, err := st.GetMonitors()
	if err != nil {
		return nil, err
	}
	out := make([]*monitor.Monitor, 0, len(list))
	for _, m := range list {
		out = append(out, m.Redacted())
	}
	return out, nil
}
