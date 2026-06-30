package web

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/keshon/beacon/internal/checks"
	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/monitorsvc"
	"github.com/keshon/beacon/internal/store"
)

var errInvalidJSON = errors.New("invalid JSON")

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
	m, err := monitorsvc.Add(st, monitorsvc.AddInput{
		Name:           req.Name,
		Type:           req.Type,
		Target:         req.Target,
		IntervalSec:    req.Interval,
		TimeoutSec:     req.Timeout,
		Retries:        req.Retries,
		HTTP:           req.HTTP,
		NotifyOverride: req.NotifyOverride,
	})
	if err != nil {
		return nil, err
	}
	return m.Redacted(), nil
}

func updateMonitorFromJSON(st *store.Store, id string, body []byte) (*monitor.Monitor, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, errInvalidJSON
	}
	patch := monitorsvc.UpdatePatch{}
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
	m, err := monitorsvc.Update(st, id, patch)
	if err != nil {
		return nil, err
	}
	return m.Redacted(), nil
}
