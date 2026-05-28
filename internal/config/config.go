package config

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

// MaxReceivers caps how many Telegram targets or Discord webhooks may be
// configured per channel (globally or per monitor override). Keeps fan-out
// predictable and stays within provider rate limits.
const MaxReceivers = 5

type Config struct {
	Listen          string              `json:"listen"`
	Auth            AuthConfig          `json:"auth"`
	Notifications   NotificationsConfig `json:"notifications"`
	Telegram        TelegramConfig      `json:"telegram"`
	Discord         DiscordConfig       `json:"discord"`
	Workers         int                 `json:"workers"`
	DefaultInterval int                 `json:"default_interval"` // seconds, 0 = 30
	Network         NetworkConfig       `json:"network"`
}

// NotificationsConfig holds global alert behavior and message templates.
type NotificationsConfig struct {
	AlertMode string           `json:"alert_mode"` // repeat | once
	Templates MessageTemplates `json:"templates"`
}

// MessageTemplates are plain-text alert bodies with {{placeholder}} variables.
// Test pings use a fixed built-in message (not configurable).
type MessageTemplates struct {
	Down      string `json:"down"`
	Recovered string `json:"recovered"`
}

// DefaultMessageTemplates returns built-in down/recovered templates.
func DefaultMessageTemplates() MessageTemplates {
	return MessageTemplates{
		Down:      "Service DOWN\n\n{{name}}\n{{message}}\nTime: {{time}}",
		Recovered: "Service RECOVERED\n\n{{name}}\n{{message}}\nTime: {{time}}",
	}
}

type NetworkConfig struct {
	Enabled      bool     `json:"enabled"`
	NodeID       string   `json:"node_id"`
	SelfURL      string   `json:"self_url"`
	Peers        []string `json:"peers"`
	SyncInterval int      `json:"sync_interval"` // seconds, default 60
	DeadTimeout  int      `json:"dead_timeout"`   // seconds, default 300
}

type AuthConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// ReceiverPolicy holds per-receiver alert mode and templates. Empty fields
// inherit from config.Notifications defaults at resolve time.
type ReceiverPolicy struct {
	AlertMode string            `json:"alert_mode,omitempty"` // repeat | once
	Templates *MessageTemplates `json:"templates,omitempty"`
}

// TelegramTarget is a single Telegram destination (bot token + chat).
type TelegramTarget struct {
	Token  string          `json:"token"`
	ChatID string          `json:"chat_id"`
	Policy *ReceiverPolicy `json:"policy,omitempty"`
}

// DiscordReceiver is a single Discord webhook destination.
type DiscordReceiver struct {
	Webhook string          `json:"webhook"`
	Policy  *ReceiverPolicy `json:"policy,omitempty"`
}

type TelegramConfig struct {
	Enabled bool             `json:"enabled"`
	Targets []TelegramTarget `json:"targets"`
}

type DiscordConfig struct {
	Enabled  bool              `json:"enabled"`
	Webhooks []DiscordReceiver `json:"webhooks"`
}

// UnmarshalJSON accepts both the new slice schema and the legacy single
// token/chat_id pair so existing beacon.json files keep working.
func (t *TelegramConfig) UnmarshalJSON(data []byte) error {
	var raw struct {
		Enabled bool             `json:"enabled"`
		Targets []TelegramTarget `json:"targets"`
		Token   string           `json:"token"`
		ChatID  string           `json:"chat_id"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	t.Enabled = raw.Enabled
	t.Targets = raw.Targets
	if len(t.Targets) == 0 && (raw.Token != "" || raw.ChatID != "") {
		t.Targets = []TelegramTarget{{Token: raw.Token, ChatID: raw.ChatID}}
	}
	return nil
}

// UnmarshalJSON accepts webhooks as strings, objects, or a legacy single webhook.
func (d *DiscordConfig) UnmarshalJSON(data []byte) error {
	var raw struct {
		Enabled  bool            `json:"enabled"`
		Webhooks json.RawMessage `json:"webhooks"`
		Webhook  string          `json:"webhook"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	d.Enabled = raw.Enabled
	if len(raw.Webhooks) > 0 {
		parsed, err := parseDiscordWebhooksJSON(raw.Webhooks)
		if err != nil {
			return err
		}
		d.Webhooks = parsed
	}
	if len(d.Webhooks) == 0 && raw.Webhook != "" {
		d.Webhooks = []DiscordReceiver{{Webhook: raw.Webhook}}
	}
	return nil
}

func parseDiscordWebhooksJSON(data json.RawMessage) ([]DiscordReceiver, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var elems []json.RawMessage
		if err := json.Unmarshal(data, &elems); err != nil {
			return nil, err
		}
		out := make([]DiscordReceiver, 0, len(elems))
		for _, elem := range elems {
			r, err := parseDiscordWebhookElement(elem)
			if err != nil {
				return nil, err
			}
			if r.Webhook != "" {
				out = append(out, r)
			}
		}
		return out, nil
	}
	r, err := parseDiscordWebhookElement(data)
	if err != nil {
		return nil, err
	}
	if r.Webhook == "" {
		return nil, nil
	}
	return []DiscordReceiver{r}, nil
}

func parseDiscordWebhookElement(data json.RawMessage) (DiscordReceiver, error) {
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "\"") {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return DiscordReceiver{}, err
		}
		return DiscordReceiver{Webhook: s}, nil
	}
	var r DiscordReceiver
	if err := json.Unmarshal(data, &r); err != nil {
		return DiscordReceiver{}, err
	}
	return r, nil
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	cfg.Normalize()
	return &cfg, nil
}

func Default() *Config {
	cfg := &Config{
		Listen:  ":8080",
		Auth:    AuthConfig{Username: "admin", Password: "admin"},
		Workers: 10,
	}
	cfg.Notifications = NotificationsConfig{
		AlertMode: "repeat",
		Templates: DefaultMessageTemplates(),
	}
	return cfg
}

func (c *Config) Normalize() {
	if c.Listen == "" {
		c.Listen = ":8080"
	}
	if c.Workers <= 0 {
		c.Workers = 10
	}
	if c.DefaultInterval < 0 {
		c.DefaultInterval = 0
	}
	if c.Network.SyncInterval <= 0 {
		c.Network.SyncInterval = 60
	}
	if c.Network.DeadTimeout <= 0 {
		c.Network.DeadTimeout = 300
	}
	if c.Network.Enabled && c.Network.NodeID == "" {
		c.Network.NodeID = uuid.New().String()
	}

	c.Telegram.Targets = sanitizeTelegramTargets(c.Telegram.Targets)
	c.Discord.Webhooks = sanitizeDiscordReceivers(c.Discord.Webhooks)
	normalizeNotifications(&c.Notifications)
}

func normalizeNotifications(n *NotificationsConfig) {
	if n == nil {
		return
	}
	mode := strings.ToLower(strings.TrimSpace(n.AlertMode))
	switch mode {
	case "once":
		n.AlertMode = "once"
	case "repeat", "":
		n.AlertMode = "repeat"
	default:
		n.AlertMode = "repeat"
	}
	def := DefaultMessageTemplates()
	if strings.TrimSpace(n.Templates.Down) == "" {
		n.Templates.Down = def.Down
	} else {
		n.Templates.Down = capTemplateField(n.Templates.Down)
	}
	if strings.TrimSpace(n.Templates.Recovered) == "" {
		n.Templates.Recovered = def.Recovered
	} else {
		n.Templates.Recovered = capTemplateField(n.Templates.Recovered)
	}
}

const maxTemplateLen = 2000

func capTemplateField(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > maxTemplateLen {
		return s[:maxTemplateLen]
	}
	return s
}

// sanitizeTelegramTargets trims whitespace, drops rows missing token or chat,
// and caps the slice at MaxReceivers.
func sanitizeTelegramTargets(in []TelegramTarget) []TelegramTarget {
	out := make([]TelegramTarget, 0, len(in))
	for _, t := range in {
		token := strings.TrimSpace(t.Token)
		chat := strings.TrimSpace(t.ChatID)
		if token == "" || chat == "" {
			continue
		}
		out = append(out, TelegramTarget{
			Token:  token,
			ChatID: chat,
			Policy: sanitizeReceiverPolicy(t.Policy),
		})
		if len(out) >= MaxReceivers {
			break
		}
	}
	return out
}

// sanitizeDiscordReceivers trims webhooks, drops empty rows, and caps at MaxReceivers.
func sanitizeDiscordReceivers(in []DiscordReceiver) []DiscordReceiver {
	out := make([]DiscordReceiver, 0, len(in))
	for _, r := range in {
		w := strings.TrimSpace(r.Webhook)
		if w == "" {
			continue
		}
		out = append(out, DiscordReceiver{
			Webhook: w,
			Policy:  sanitizeReceiverPolicy(r.Policy),
		})
		if len(out) >= MaxReceivers {
			break
		}
	}
	return out
}

// sanitizeReceiverPolicy trims alert policy on a receiver row.
func sanitizeReceiverPolicy(p *ReceiverPolicy) *ReceiverPolicy {
	if p == nil {
		return nil
	}
	out := &ReceiverPolicy{}
	if mode := strings.ToLower(strings.TrimSpace(p.AlertMode)); mode == "once" || mode == "repeat" {
		out.AlertMode = mode
	}
	if p.Templates != nil {
		tpl := MessageTemplates{
			Down:      capTemplateField(strings.TrimSpace(p.Templates.Down)),
			Recovered: capTemplateField(strings.TrimSpace(p.Templates.Recovered)),
		}
		if tpl.Down != "" || tpl.Recovered != "" {
			out.Templates = &tpl
		}
	}
	if out.AlertMode == "" && out.Templates == nil {
		return nil
	}
	return out
}

// DefaultIntervalDuration returns the global check interval.
func (c *Config) DefaultIntervalDuration() time.Duration {
	if c.DefaultInterval > 0 {
		return time.Duration(c.DefaultInterval) * time.Second
	}
	return 30 * time.Second
}
