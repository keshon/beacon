package monitorsvc

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/keshon/beacon/internal/checks"
	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/store"
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
func Add(st *store.Store, in AddInput) (*monitor.Monitor, error) {
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
