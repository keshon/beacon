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
	Listen          string         `json:"listen"`
	Auth            AuthConfig     `json:"auth"`
	Telegram        TelegramConfig `json:"telegram"`
	Discord         DiscordConfig  `json:"discord"`
	Workers         int            `json:"workers"`
	DefaultInterval int            `json:"default_interval"` // seconds, 0 = 30
	Network         NetworkConfig  `json:"network"`
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

// TelegramTarget is a single Telegram destination (bot token + chat).
type TelegramTarget struct {
	Token  string `json:"token"`
	ChatID string `json:"chat_id"`
}

type TelegramConfig struct {
	Enabled bool             `json:"enabled"`
	Targets []TelegramTarget `json:"targets"`
}

type DiscordConfig struct {
	Enabled  bool     `json:"enabled"`
	Webhooks []string `json:"webhooks"`
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

// UnmarshalJSON accepts both the new webhooks slice and the legacy single
// webhook string.
func (d *DiscordConfig) UnmarshalJSON(data []byte) error {
	var raw struct {
		Enabled  bool     `json:"enabled"`
		Webhooks []string `json:"webhooks"`
		Webhook  string   `json:"webhook"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	d.Enabled = raw.Enabled
	d.Webhooks = raw.Webhooks
	if len(d.Webhooks) == 0 && raw.Webhook != "" {
		d.Webhooks = []string{raw.Webhook}
	}
	return nil
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
	return &Config{
		Listen:  ":8080",
		Auth:    AuthConfig{Username: "admin", Password: "admin"},
		Workers: 10,
	}
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
	c.Discord.Webhooks = sanitizeWebhooks(c.Discord.Webhooks)
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
		out = append(out, TelegramTarget{Token: token, ChatID: chat})
		if len(out) >= MaxReceivers {
			break
		}
	}
	return out
}

// sanitizeWebhooks trims whitespace, drops empty entries, and caps at
// MaxReceivers.
func sanitizeWebhooks(in []string) []string {
	out := make([]string, 0, len(in))
	for _, w := range in {
		w = strings.TrimSpace(w)
		if w == "" {
			continue
		}
		out = append(out, w)
		if len(out) >= MaxReceivers {
			break
		}
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
