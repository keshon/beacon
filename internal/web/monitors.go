package web

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/keshon/beacon/internal/checks"
	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/store"
)

var errInvalidJSON = errors.New("invalid JSON")

func listMonitorsRedacted(st *store.Store) ([]*monitor.Monitor, error) {
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

type addMonitorInput struct {
	Name           string
	Type           string
	Target         string
	IntervalSec    int
	TimeoutSec     int
	Retries        int
	HTTP           *checks.HTTPOptions
	NotifyOverride *monitor.NotifyOverride
}

func addMonitor(st *store.Store, in addMonitorInput) (*monitor.Monitor, error) {
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
	return m.Redacted(), nil
}

func addMonitorFromJSON(st *store.Store, body []byte) (*monitor.Monitor, error) {
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
		return nil, errInvalidJSON
	}
	return addMonitor(st, addMonitorInput{
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

type updateMonitorPatch struct {
	Enabled        *bool
	Name           *string
	Type           *string
	Target         *string
	IntervalSec    *int
	HTTP           *checks.HTTPOptions
	NotifyOverride *monitor.NotifyOverride
}

func updateMonitor(st *store.Store, id string, patch updateMonitorPatch) (*monitor.Monitor, error) {
	mon, err := st.UpdateMonitor(id, func(mon *monitor.Monitor) error {
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
	if err != nil {
		return nil, err
	}
	return mon.Redacted(), nil
}

func updateMonitorFromJSON(st *store.Store, id string, body []byte) (*monitor.Monitor, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, errInvalidJSON
	}
	patch := updateMonitorPatch{}
	if v, ok := raw["enabled"]; ok {
		var b bool
		if err := json.Unmarshal(v, &b); err != nil {
			return nil, errInvalidJSON
		}
		patch.Enabled = &b
	}
	if v, ok := raw["name"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return nil, errInvalidJSON
		}
		patch.Name = &s
	}
	if v, ok := raw["type"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return nil, errInvalidJSON
		}
		patch.Type = &s
	}
	if v, ok := raw["target"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return nil, errInvalidJSON
		}
		patch.Target = &s
	}
	if v, ok := raw["interval"]; ok {
		var n int
		if err := json.Unmarshal(v, &n); err != nil {
			return nil, errInvalidJSON
		}
		patch.IntervalSec = &n
	}
	if v, ok := raw["http"]; ok {
		var h checks.HTTPOptions
		if err := json.Unmarshal(v, &h); err != nil {
			return nil, errInvalidJSON
		}
		patch.HTTP = &h
	}
	if v, ok := raw["notify_override"]; ok {
		trimmed := strings.TrimSpace(string(v))
		if trimmed != "" && trimmed != "null" && trimmed != "{}" {
			var no monitor.NotifyOverride
			if err := json.Unmarshal(v, &no); err != nil {
				return nil, errInvalidJSON
			}
			patch.NotifyOverride = &no
		}
	}
	return updateMonitor(st, id, patch)
}
