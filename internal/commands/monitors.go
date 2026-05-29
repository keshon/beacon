package commands

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/store"
	"github.com/keshon/commandkit"
)

type MonitorListCmd struct {
	store *store.Store
}

func (c *MonitorListCmd) Name() string        { return "monitor:list" }
func (c *MonitorListCmd) Description() string { return "List all monitors" }

func (c *MonitorListCmd) Run(ctx context.Context, inv *commandkit.Invocation) error {
	if d := getHTTPData(inv); d != nil {
		list, err := c.store.GetMonitors()
		if err != nil {
			http.Error(d.W, err.Error(), http.StatusInternalServerError)
			return nil
		}
		writeJSONTo(d.W, list)
		return nil
	}
	if d := getCLIData(inv); d != nil {
		list, err := c.store.GetMonitors()
		if err != nil {
			return err
		}
		writeJSONToWriter(d.Out, list)
		return nil
	}
	return nil
}

type MonitorAddCmd struct {
	store *store.Store
}

func (c *MonitorAddCmd) Name() string        { return "monitor:add" }
func (c *MonitorAddCmd) Description() string { return "Add a monitor" }

func (c *MonitorAddCmd) Run(ctx context.Context, inv *commandkit.Invocation) error {
	var m *monitor.Monitor
	if d := getHTTPData(inv); d != nil {
		var req struct {
			Name           string                  `json:"name"`
			Type           string                  `json:"type"`
			Target         string                  `json:"target"`
			Interval       int                     `json:"interval"`
			Timeout        int                     `json:"timeout"`
			Retries        int                     `json:"retries"`
			NotifyOverride *monitor.NotifyOverride `json:"notify_override"`
		}
		if err := json.NewDecoder(d.R.Body).Decode(&req); err != nil {
			http.Error(d.W, "invalid JSON", http.StatusBadRequest)
			return nil
		}
		typ, err := monitor.NormalizeType(req.Type)
		if err != nil {
			http.Error(d.W, err.Error(), http.StatusBadRequest)
			return nil
		}
		if err := monitor.ValidateTarget(typ, req.Target); err != nil {
			http.Error(d.W, err.Error(), http.StatusBadRequest)
			return nil
		}
		m = &monitor.Monitor{
			ID: uuid.New().String(), Name: strings.TrimSpace(req.Name), Type: typ, Target: strings.TrimSpace(req.Target),
			Interval: 0, Timeout: 10 * time.Second, Retries: 3, Enabled: true,
		}
		if req.Interval > 0 {
			m.Interval = time.Duration(req.Interval) * time.Second
		}
		if req.Timeout > 0 {
			m.Timeout = time.Duration(req.Timeout) * time.Second
		}
		if req.Retries > 0 {
			m.Retries = req.Retries
		}
		if req.NotifyOverride != nil {
			m.NotifyOverride = monitor.SanitizeNotifyOverride(req.NotifyOverride)
		}
		var cfg config.Config
		if ok, err := c.store.GetConfig(&cfg); err == nil && ok && cfg.Network.Enabled && cfg.Network.NodeID != "" {
			m.OwnerNodeID = cfg.Network.NodeID
		}
		if err := c.store.SetMonitor(m); err != nil {
			http.Error(d.W, err.Error(), http.StatusInternalServerError)
			return nil
		}
		writeJSONTo(d.W, m)
		return nil
	}
	if d := getCLIData(inv); d != nil {
		name := d.Args["name"]
		typ := d.Args["type"]
		target := d.Args["target"]
		typ, err := monitor.NormalizeType(typ)
		if err != nil {
			return err
		}
		if err := monitor.ValidateTarget(typ, target); err != nil {
			return err
		}
		m = &monitor.Monitor{
			ID: uuid.New().String(), Name: name, Type: typ, Target: strings.TrimSpace(target),
			Interval: 30 * time.Second, Timeout: 10 * time.Second, Retries: 3, Enabled: true,
		}
		if err := c.store.SetMonitor(m); err != nil {
			return err
		}
		writeJSONToWriter(d.Out, m)
		return nil
	}
	return nil
}

type MonitorDeleteCmd struct {
	store *store.Store
}

func (c *MonitorDeleteCmd) Name() string        { return "monitor:delete" }
func (c *MonitorDeleteCmd) Description() string { return "Delete a monitor" }

func (c *MonitorDeleteCmd) Run(ctx context.Context, inv *commandkit.Invocation) error {
	var id string
	if d := getHTTPData(inv); d != nil {
		id = d.PathID
		if id == "" {
			id = strings.TrimPrefix(strings.TrimSuffix(d.R.URL.Path, "/"), "/api/monitors/")
		}
		if id == "" {
			http.Error(d.W, "missing id", http.StatusBadRequest)
			return nil
		}
		if err := c.store.DeleteMonitor(id); err != nil {
			http.Error(d.W, err.Error(), http.StatusInternalServerError)
			return nil
		}
		d.W.WriteHeader(http.StatusNoContent)
		return nil
	}
	if d := getCLIData(inv); d != nil {
		id = d.Args["id"]
		if id == "" {
			return nil
		}
		return c.store.DeleteMonitor(id)
	}
	return nil
}

type MonitorUpdateCmd struct {
	store *store.Store
}

func (c *MonitorUpdateCmd) Name() string        { return "monitor:update" }
func (c *MonitorUpdateCmd) Description() string { return "Update a monitor" }

func (c *MonitorUpdateCmd) Run(ctx context.Context, inv *commandkit.Invocation) error {
	var id string
	var enabled *bool
	if d := getHTTPData(inv); d != nil {
		id = d.PathID
		if id == "" {
			id = strings.TrimPrefix(strings.TrimSuffix(d.R.URL.Path, "/"), "/api/monitors/")
		}
		if id == "" {
			http.Error(d.W, "missing id", http.StatusBadRequest)
			return nil
		}
		var patch struct {
			Enabled        *bool                   `json:"enabled"`
			Name           *string                 `json:"name"`
			Type           *string                 `json:"type"`
			Target         *string                 `json:"target"`
			Interval       *int                    `json:"interval"`
			NotifyOverride *monitor.NotifyOverride `json:"notify_override"`
		}
		if err := json.NewDecoder(d.R.Body).Decode(&patch); err != nil {
			http.Error(d.W, "invalid JSON", http.StatusBadRequest)
			return nil
		}
		enabled = patch.Enabled
		mon, err := c.store.GetMonitor(id)
		if err != nil {
			http.Error(d.W, err.Error(), http.StatusInternalServerError)
			return nil
		}
		if mon == nil {
			http.Error(d.W, "not found", http.StatusNotFound)
			return nil
		}
		if enabled != nil {
			mon.Enabled = *enabled
		}
		if patch.Name != nil {
			mon.Name = *patch.Name
		}
		if patch.Type != nil {
			typ, err := monitor.NormalizeType(*patch.Type)
			if err != nil {
				http.Error(d.W, err.Error(), http.StatusBadRequest)
				return nil
			}
			mon.Type = typ
		}
		if patch.Target != nil {
			mon.Target = strings.TrimSpace(*patch.Target)
		}
		if patch.Type != nil || patch.Target != nil {
			if err := monitor.ValidateTarget(mon.Type, mon.Target); err != nil {
				http.Error(d.W, err.Error(), http.StatusBadRequest)
				return nil
			}
		}
		if patch.Interval != nil {
			if *patch.Interval > 0 {
				mon.Interval = time.Duration(*patch.Interval) * time.Second
			} else {
				mon.Interval = 0
			}
		}
		if patch.NotifyOverride != nil {
			mon.NotifyOverride = monitor.SanitizeNotifyOverride(patch.NotifyOverride)
		}
		if err := c.store.SetMonitor(mon); err != nil {
			http.Error(d.W, err.Error(), http.StatusInternalServerError)
			return nil
		}
		writeJSONTo(d.W, mon)
		return nil
	}
	if d := getCLIData(inv); d != nil {
		id = d.Args["id"]
		if id == "" {
			return nil
		}
		switch d.Args["enabled"] {
		case "true":
			b := true
			enabled = &b
		case "false":
			b := false
			enabled = &b
		}
		mon, err := c.store.GetMonitor(id)
		if err != nil {
			return err
		}
		if mon == nil {
			return nil
		}
		if enabled != nil {
			mon.Enabled = *enabled
		}
		if err := c.store.SetMonitor(mon); err != nil {
			return err
		}
		writeJSONToWriter(d.Out, mon)
		return nil
	}
	return nil
}
