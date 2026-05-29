package monitor

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/keshon/beacon/internal/config"
)

// NotifyOverride holds per-monitor notification overrides. When a slice is
// non-empty it fully replaces the matching global channel.
type NotifyOverride struct {
	Telegram  []config.TelegramTarget  `json:"telegram,omitempty"`
	Discord   []config.DiscordReceiver `json:"discord,omitempty"`
	AlertMode string                   `json:"alert_mode,omitempty"` // deprecated; migrated to row policy
	Templates *config.MessageTemplates `json:"templates,omitempty"`  // deprecated; migrated to row policy
}

// UnmarshalJSON accepts slice schemas and legacy single-object shapes.
func (n *NotifyOverride) UnmarshalJSON(data []byte) error {
	*n = NotifyOverride{}
	var raw struct {
		Telegram  json.RawMessage          `json:"telegram"`
		Discord   json.RawMessage          `json:"discord"`
		AlertMode string                   `json:"alert_mode"`
		Templates *config.MessageTemplates `json:"templates"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	n.AlertMode = raw.AlertMode
	n.Templates = raw.Templates
	if len(raw.Telegram) > 0 {
		trimmed := strings.TrimSpace(string(raw.Telegram))
		if strings.HasPrefix(trimmed, "[") {
			if err := json.Unmarshal(raw.Telegram, &n.Telegram); err != nil {
				return err
			}
		} else if trimmed != "null" {
			var single config.TelegramTarget
			if err := json.Unmarshal(raw.Telegram, &single); err != nil {
				return err
			}
			if single.Token != "" || single.ChatID != "" {
				n.Telegram = []config.TelegramTarget{single}
			}
		}
	}
	if len(raw.Discord) > 0 {
		parsed, err := config.ParseDiscordReceiversJSON(raw.Discord)
		if err != nil {
			return err
		}
		if len(parsed) > 0 {
			n.Discord = parsed
		}
	}
	MigrateNotifyOverride(n)
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
