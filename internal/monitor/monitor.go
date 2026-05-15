package monitor

import (
	"encoding/json"
	"strings"
	"time"
)

// TelegramTarget mirrors config.TelegramTarget but lives here to avoid an
// import cycle between monitor and config.
type TelegramTarget struct {
	Token  string `json:"token,omitempty"`
	ChatID string `json:"chat_id,omitempty"`
}

// NotifyOverride holds per-monitor notification overrides. When a slice is
// non-empty it fully replaces the matching global channel.
type NotifyOverride struct {
	Telegram []TelegramTarget `json:"telegram,omitempty"`
	Discord  []string         `json:"discord,omitempty"`
}

// UnmarshalJSON accepts both the new slice schema and the legacy single-object
// shape (`telegram: {token, chat_id}`, `discord: {webhook}`).
func (n *NotifyOverride) UnmarshalJSON(data []byte) error {
	var raw struct {
		Telegram json.RawMessage `json:"telegram"`
		Discord  json.RawMessage `json:"discord"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw.Telegram) > 0 {
		trimmed := strings.TrimSpace(string(raw.Telegram))
		if strings.HasPrefix(trimmed, "[") {
			if err := json.Unmarshal(raw.Telegram, &n.Telegram); err != nil {
				return err
			}
		} else if trimmed != "null" {
			var single TelegramTarget
			if err := json.Unmarshal(raw.Telegram, &single); err != nil {
				return err
			}
			if single.Token != "" || single.ChatID != "" {
				n.Telegram = []TelegramTarget{single}
			}
		}
	}
	if len(raw.Discord) > 0 {
		trimmed := strings.TrimSpace(string(raw.Discord))
		if strings.HasPrefix(trimmed, "[") {
			if err := json.Unmarshal(raw.Discord, &n.Discord); err != nil {
				return err
			}
		} else if trimmed != "null" {
			var single struct {
				Webhook string `json:"webhook"`
			}
			if err := json.Unmarshal(raw.Discord, &single); err != nil {
				return err
			}
			if single.Webhook != "" {
				n.Discord = []string{single.Webhook}
			}
		}
	}
	return nil
}

type Monitor struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Type           string          `json:"type"` // http, tcp
	Target         string          `json:"target"`
	Interval       time.Duration   `json:"interval"`
	Timeout        time.Duration   `json:"timeout"`
	Retries        int             `json:"retries"`
	Enabled        bool            `json:"enabled"`
	Notify         []string        `json:"notify"` // telegram, discord
	NotifyOverride *NotifyOverride `json:"notify_override,omitempty"`
	OwnerNodeID    string          `json:"owner_node_id,omitempty"` // empty = local/legacy
}
