package config

import (
	"encoding/json"
	"os"
	"time"

	"github.com/google/uuid"
)

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

type TelegramConfig struct {
	Enabled bool   `json:"enabled"`
	Token   string `json:"token"`
	ChatID  string `json:"chat_id"`
}

type DiscordConfig struct {
	Enabled  bool   `json:"enabled"`
	Webhook  string `json:"webhook"`
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
	if cfg.Listen == "" {
		cfg.Listen = ":8080"
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 10
	}
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
}

// DefaultIntervalDuration returns the global check interval.
func (c *Config) DefaultIntervalDuration() time.Duration {
	if c.DefaultInterval > 0 {
		return time.Duration(c.DefaultInterval) * time.Second
	}
	return 30 * time.Second
}
