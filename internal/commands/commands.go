package commands

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/store"
	"github.com/keshon/commandkit"
)

// HTTPData carries the HTTP request context for command execution.
type HTTPData struct {
	W      http.ResponseWriter
	R      *http.Request
	PathID string // Set when using path param {id}
}

// RegisterAll registers all commands with the default registry.
func RegisterAll(st *store.Store) {
	commandkit.DefaultRegistry.Register(&MonitorListCmd{store: st})
	commandkit.DefaultRegistry.Register(&MonitorAddCmd{store: st})
	commandkit.DefaultRegistry.Register(&MonitorDeleteCmd{store: st})
	commandkit.DefaultRegistry.Register(&MonitorUpdateCmd{store: st})
	commandkit.DefaultRegistry.Register(&StateGetCmd{store: st})
	commandkit.DefaultRegistry.Register(&EventsGetCmd{store: st})
}

// RegisterConfigCommands registers config commands. cfg is updated in-place when config is saved.
func RegisterConfigCommands(st *store.Store, cfg *config.Config) {
	commandkit.DefaultRegistry.Register(&ConfigGetCmd{store: st})
	commandkit.DefaultRegistry.Register(&ConfigSetCmd{store: st, cfg: cfg})
}

func getHTTPData(inv *commandkit.Invocation) *HTTPData {
	if inv == nil || inv.Data == nil {
		return nil
	}
	d, ok := inv.Data.(*HTTPData)
	if !ok {
		return nil
	}
	return d
}

func writeJSONTo(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeJSONToWriter(out io.Writer, v any) {
	json.NewEncoder(out).Encode(v)
}

// --- MonitorListCmd ---

type MonitorListCmd struct {
	store *store.Store
}

func (c *MonitorListCmd) Name() string        { return "monitor:list" }
func (c *MonitorListCmd) Description() string { return "List all monitors" }

func (c *MonitorListCmd) Run(ctx context.Context, inv *commandkit.Invocation) error {
	if d := getHTTPData(inv); d != nil {
		writeJSONTo(d.W, c.store.GetMonitors())
		return nil
	}
	if d := getCLIData(inv); d != nil {
		writeJSONToWriter(d.Out, c.store.GetMonitors())
		return nil
	}
	return nil
}

// --- MonitorAddCmd ---

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
		m = &monitor.Monitor{
			ID: uuid.New().String(), Name: req.Name, Type: req.Type, Target: req.Target,
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
			m.NotifyOverride = req.NotifyOverride
		}
		var cfg config.Config
		if ok, _ := c.store.GetConfig(&cfg); ok && cfg.Network.Enabled && cfg.Network.NodeID != "" {
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
		if typ == "" {
			typ = "http"
		}
		m = &monitor.Monitor{
			ID: uuid.New().String(), Name: name, Type: typ, Target: target,
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

// --- MonitorDeleteCmd ---

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

// --- MonitorUpdateCmd ---

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
		m := c.store.GetMonitor(id)
		if m == nil {
			http.Error(d.W, "not found", http.StatusNotFound)
			return nil
		}
		if enabled != nil {
			m.Enabled = *enabled
		}
		if patch.Name != nil {
			m.Name = *patch.Name
		}
		if patch.Type != nil && (*patch.Type == "http" || *patch.Type == "tcp") {
			m.Type = *patch.Type
		}
		if patch.Target != nil {
			m.Target = *patch.Target
		}
		if patch.Interval != nil {
			if *patch.Interval > 0 {
				m.Interval = time.Duration(*patch.Interval) * time.Second
			} else {
				m.Interval = 0
			}
		}
		if patch.NotifyOverride != nil {
			m.NotifyOverride = patch.NotifyOverride
		}
		if err := c.store.SetMonitor(m); err != nil {
			http.Error(d.W, err.Error(), http.StatusInternalServerError)
			return nil
		}
		writeJSONTo(d.W, m)
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
		m := c.store.GetMonitor(id)
		if m == nil {
			return nil
		}
		if enabled != nil {
			m.Enabled = *enabled
		}
		if err := c.store.SetMonitor(m); err != nil {
			return err
		}
		writeJSONToWriter(d.Out, m)
		return nil
	}
	return nil
}

// --- StateGetCmd ---

type StateGetCmd struct {
	store *store.Store
}

func (c *StateGetCmd) Name() string        { return "state:get" }
func (c *StateGetCmd) Description() string { return "Get all monitor state" }

func (c *StateGetCmd) Run(ctx context.Context, inv *commandkit.Invocation) error {
	if d := getHTTPData(inv); d != nil {
		writeJSONTo(d.W, c.store.GetAllState())
		return nil
	}
	if d := getCLIData(inv); d != nil {
		writeJSONToWriter(d.Out, c.store.GetAllState())
		return nil
	}
	return nil
}

// --- EventsGetCmd ---

type EventsGetCmd struct {
	store *store.Store
}

func (c *EventsGetCmd) Name() string        { return "events:get" }
func (c *EventsGetCmd) Description() string { return "Get events" }

func (c *EventsGetCmd) Run(ctx context.Context, inv *commandkit.Invocation) error {
	limit := 100
	if d := getHTTPData(inv); d != nil {
		if l := d.R.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 {
				limit = n
			}
		}
		writeJSONTo(d.W, c.store.GetEvents(limit))
		return nil
	}
	if d := getCLIData(inv); d != nil {
		if l := d.Args["limit"]; l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 {
				limit = n
			}
		}
		writeJSONToWriter(d.Out, c.store.GetEvents(limit))
		return nil
	}
	return nil
}

// --- ConfigGetCmd ---

type ConfigGetCmd struct {
	store *store.Store
}

func (c *ConfigGetCmd) Name() string        { return "config:get" }
func (c *ConfigGetCmd) Description() string { return "Get config" }

func (c *ConfigGetCmd) Run(ctx context.Context, inv *commandkit.Invocation) error {
	if d := getHTTPData(inv); d != nil {
		var cfg config.Config
		ok, _ := c.store.GetConfig(&cfg)
		if !ok {
			cfg = *config.Default()
		}
		writeJSONTo(d.W, cfg)
		return nil
	}
	return nil
}

// --- ConfigSetCmd ---

type ConfigSetCmd struct {
	store *store.Store
	cfg   *config.Config
}

func (c *ConfigSetCmd) Name() string        { return "config:set" }
func (c *ConfigSetCmd) Description() string { return "Save config" }

func (c *ConfigSetCmd) Run(ctx context.Context, inv *commandkit.Invocation) error {
	d := getHTTPData(inv)
	if d == nil {
		return nil
	}
	var cfg config.Config
	if err := json.NewDecoder(d.R.Body).Decode(&cfg); err != nil {
		http.Error(d.W, "invalid JSON", http.StatusBadRequest)
		return nil
	}
	cfg.Normalize()
	if err := c.store.SetConfig(&cfg); err != nil {
		http.Error(d.W, err.Error(), http.StatusInternalServerError)
		return nil
	}
	*c.cfg = cfg
	writeJSONTo(d.W, cfg)
	return nil
}
