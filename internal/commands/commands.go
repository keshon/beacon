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

// sanitizeNotifyOverride drops incomplete rows, trims whitespace, and caps
// each channel at config.MaxReceivers. Returns nil when override is empty.
func sanitizeNotifyOverride(in *monitor.NotifyOverride) *monitor.NotifyOverride {
	if in == nil {
		return nil
	}
	monitor.MigrateNotifyOverride(in)
	out := &monitor.NotifyOverride{}
	for _, t := range in.Telegram {
		token := strings.TrimSpace(t.Token)
		chat := strings.TrimSpace(t.ChatID)
		if token == "" || chat == "" {
			continue
		}
		out.Telegram = append(out.Telegram, monitor.TelegramTarget{
			Token:  token,
			ChatID: chat,
			Policy: sanitizeReceiverPolicy(t.Policy),
		})
		if len(out.Telegram) >= config.MaxReceivers {
			break
		}
	}
	for _, d := range in.Discord {
		w := strings.TrimSpace(d.Webhook)
		if w == "" {
			continue
		}
		out.Discord = append(out.Discord, monitor.DiscordReceiver{
			Webhook: w,
			Policy:  sanitizeReceiverPolicy(d.Policy),
		})
		if len(out.Discord) >= config.MaxReceivers {
			break
		}
	}
	if len(out.Telegram) == 0 && len(out.Discord) == 0 {
		return nil
	}
	return out
}

func sanitizeReceiverPolicy(p *config.ReceiverPolicy) *config.ReceiverPolicy {
	if p == nil {
		return nil
	}
	out := &config.ReceiverPolicy{}
	if mode := strings.ToLower(strings.TrimSpace(p.AlertMode)); mode == "repeat" || mode == "once" {
		out.AlertMode = mode
	}
	if p.Templates != nil {
		san := sanitizeOverrideTemplates(p.Templates)
		if san.Down != "" || san.Recovered != "" {
			out.Templates = &san
		}
	}
	if out.AlertMode == "" && out.Templates == nil {
		return nil
	}
	return out
}

func sanitizeOverrideTemplates(t *config.MessageTemplates) config.MessageTemplates {
	if t == nil {
		return config.MessageTemplates{}
	}
	cap := func(s string) string {
		s = strings.TrimSpace(s)
		if len(s) > 2000 {
			return s[:2000]
		}
		return s
	}
	return config.MessageTemplates{
		Down:      cap(t.Down),
		Recovered: cap(t.Recovered),
	}
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
			m.NotifyOverride = sanitizeNotifyOverride(req.NotifyOverride)
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
		m, err := c.store.GetMonitor(id)
		if err != nil {
			http.Error(d.W, err.Error(), http.StatusInternalServerError)
			return nil
		}
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
		if patch.Type != nil {
			typ, err := monitor.NormalizeType(*patch.Type)
			if err != nil {
				http.Error(d.W, err.Error(), http.StatusBadRequest)
				return nil
			}
			m.Type = typ
		}
		if patch.Target != nil {
			m.Target = strings.TrimSpace(*patch.Target)
		}
		if patch.Type != nil || patch.Target != nil {
			if err := monitor.ValidateTarget(m.Type, m.Target); err != nil {
				http.Error(d.W, err.Error(), http.StatusBadRequest)
				return nil
			}
		}
		if patch.Interval != nil {
			if *patch.Interval > 0 {
				m.Interval = time.Duration(*patch.Interval) * time.Second
			} else {
				m.Interval = 0
			}
		}
		if patch.NotifyOverride != nil {
			m.NotifyOverride = sanitizeNotifyOverride(patch.NotifyOverride)
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
		m, err := c.store.GetMonitor(id)
		if err != nil {
			return err
		}
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
		st, err := c.store.GetAllState()
		if err != nil {
			http.Error(d.W, err.Error(), http.StatusInternalServerError)
			return nil
		}
		writeJSONTo(d.W, st)
		return nil
	}
	if d := getCLIData(inv); d != nil {
		st, err := c.store.GetAllState()
		if err != nil {
			return err
		}
		writeJSONToWriter(d.Out, st)
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
		events, err := c.store.GetEvents(limit)
		if err != nil {
			http.Error(d.W, err.Error(), http.StatusInternalServerError)
			return nil
		}
		writeJSONTo(d.W, events)
		return nil
	}
	if d := getCLIData(inv); d != nil {
		if l := d.Args["limit"]; l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 {
				limit = n
			}
		}
		events, err := c.store.GetEvents(limit)
		if err != nil {
			return err
		}
		writeJSONToWriter(d.Out, events)
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
		ok, err := c.store.GetConfig(&cfg)
		if err != nil {
			http.Error(d.W, err.Error(), http.StatusInternalServerError)
			return nil
		}
		if !ok {
			cfg = *config.Default()
		}
		cfg.Normalize()
		writeJSONTo(d.W, cfg.ToPublic())
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
	var incoming config.Config
	if err := json.NewDecoder(d.R.Body).Decode(&incoming); err != nil {
		http.Error(d.W, "invalid JSON", http.StatusBadRequest)
		return nil
	}
	var existing config.Config
	ok, err := c.store.GetConfig(&existing)
	if err != nil {
		http.Error(d.W, err.Error(), http.StatusInternalServerError)
		return nil
	}
	if !ok {
		existing = *config.Default()
	}
	config.ApplyNonSecret(&existing, &incoming)
	if err := config.MergeSecrets(&existing, &incoming); err != nil {
		http.Error(d.W, err.Error(), http.StatusBadRequest)
		return nil
	}
	existing.Normalize()
	if err := existing.Auth.EnsureAuthHashed(); err != nil {
		http.Error(d.W, err.Error(), http.StatusInternalServerError)
		return nil
	}
	if err := c.store.SetConfig(&existing); err != nil {
		http.Error(d.W, err.Error(), http.StatusInternalServerError)
		return nil
	}
	*c.cfg = existing
	writeJSONTo(d.W, existing.ToPublic())
	return nil
}
