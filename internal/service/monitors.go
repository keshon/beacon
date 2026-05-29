package service

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/keshon/beacon/internal/checks"
	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/store"
)

func ListMonitors(st *store.Store) ([]*monitor.Monitor, error) {
	return st.GetMonitors()
}

type AddMonitorInput struct {
	Name           string
	Type           string
	Target         string
	IntervalSec    int
	TimeoutSec     int
	Retries        int
	HTTP           *checks.HTTPOptions
	NotifyOverride *monitor.NotifyOverride
}

func AddMonitor(st *store.Store, in AddMonitorInput) (*monitor.Monitor, error) {
	typ, err := monitor.NormalizeType(in.Type)
	if err != nil {
		return nil, err
	}
	target := strings.TrimSpace(in.Target)
	if err := monitor.ValidateTarget(typ, target); err != nil {
		return nil, err
	}
	m := &monitor.Monitor{
		ID:       uuid.New().String(),
		Name:     strings.TrimSpace(in.Name),
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

func AddMonitorFromJSON(st *store.Store, body []byte) (*monitor.Monitor, error) {
	var req struct {
		Name           string                  `json:"name"`
		Type           string                  `json:"type"`
		Target         string                  `json:"target"`
		Interval       int                     `json:"interval"`
		Timeout        int                     `json:"timeout"`
		Retries        int                     `json:"retries"`
		HTTP           *checks.HTTPOptions     `json:"http,omitempty"`
		NotifyOverride *monitor.NotifyOverride `json:"notify_override"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, ErrInvalidJSON
	}
	return AddMonitor(st, AddMonitorInput{
		Name:           req.Name,
		Type:           req.Type,
		Target:         req.Target,
		IntervalSec:    req.Interval,
		TimeoutSec:     req.Timeout,
		Retries:        req.Retries,
		HTTP:           req.HTTP,
		NotifyOverride: req.NotifyOverride,
	})
}

type UpdateMonitorPatch struct {
	Enabled        *bool
	Name           *string
	Type           *string
	Target         *string
	IntervalSec    *int
	HTTP           *checks.HTTPOptions
	NotifyOverride *monitor.NotifyOverride
}

func UpdateMonitor(st *store.Store, id string, patch UpdateMonitorPatch) (*monitor.Monitor, error) {
	mon, err := st.GetMonitor(id)
	if err != nil {
		return nil, err
	}
	if mon == nil {
		return nil, ErrMonitorNotFound
	}
	if patch.Enabled != nil {
		mon.Enabled = *patch.Enabled
	}
	if patch.Name != nil {
		mon.Name = *patch.Name
	}
	if patch.Type != nil {
		typ, err := monitor.NormalizeType(*patch.Type)
		if err != nil {
			return nil, err
		}
		mon.Type = typ
	}
	if patch.Target != nil {
		mon.Target = strings.TrimSpace(*patch.Target)
	}
	if patch.Type != nil || patch.Target != nil {
		if err := monitor.ValidateTarget(mon.Type, mon.Target); err != nil {
			return nil, err
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
		mon.NotifyOverride = monitor.SanitizeNotifyOverride(patch.NotifyOverride)
	}
	if err := st.SetMonitor(mon); err != nil {
		return nil, err
	}
	return mon, nil
}

func UpdateMonitorFromJSON(st *store.Store, id string, body []byte) (*monitor.Monitor, error) {
	var patch struct {
		Enabled        *bool                   `json:"enabled"`
		Name           *string                 `json:"name"`
		Type           *string                 `json:"type"`
		Target         *string                 `json:"target"`
		Interval       *int                    `json:"interval"`
		HTTP           *checks.HTTPOptions     `json:"http,omitempty"`
		NotifyOverride *monitor.NotifyOverride `json:"notify_override"`
	}
	if err := json.Unmarshal(body, &patch); err != nil {
		return nil, ErrInvalidJSON
	}
	return UpdateMonitor(st, id, UpdateMonitorPatch{
		Enabled:        patch.Enabled,
		Name:           patch.Name,
		Type:           patch.Type,
		Target:         patch.Target,
		IntervalSec:    patch.Interval,
		HTTP:           patch.HTTP,
		NotifyOverride: patch.NotifyOverride,
	})
}

func DeleteMonitor(st *store.Store, id string) error {
	return st.DeleteMonitor(id)
}
