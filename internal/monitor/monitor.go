package monitor

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/keshon/beacon/internal/config"
)

// TelegramTarget mirrors config.TelegramTarget for per-monitor overrides.
type TelegramTarget struct {
	Token  string                 `json:"token,omitempty"`
	ChatID string                 `json:"chat_id,omitempty"`
	Policy *config.ReceiverPolicy `json:"policy,omitempty"`
}

// DiscordReceiver mirrors config.DiscordReceiver for per-monitor overrides.
type DiscordReceiver struct {
	Webhook string                 `json:"webhook,omitempty"`
	Policy  *config.ReceiverPolicy `json:"policy,omitempty"`
}

// NotifyOverride holds per-monitor notification overrides. When a slice is
// non-empty it fully replaces the matching global channel.
type NotifyOverride struct {
	Telegram  []TelegramTarget         `json:"telegram,omitempty"`
	Discord   []DiscordReceiver        `json:"discord,omitempty"`
	AlertMode string                   `json:"alert_mode,omitempty"` // deprecated; migrated to row policy
	Templates *config.MessageTemplates `json:"templates,omitempty"`  // deprecated; migrated to row policy
}

// UnmarshalJSON accepts slice schemas and legacy single-object shapes.
func (n *NotifyOverride) UnmarshalJSON(data []byte) error {
	*n = NotifyOverride{}
	var raw struct {
		Telegram  json.RawMessage        `json:"telegram"`
		Discord   json.RawMessage        `json:"discord"`
		AlertMode string                 `json:"alert_mode"`
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
		parsed, err := parseOverrideDiscordJSON(raw.Discord)
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

func parseOverrideDiscordJSON(data json.RawMessage) ([]DiscordReceiver, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}
	if strings.HasPrefix(trimmed, "{") {
		var single struct {
			Webhook string `json:"webhook"`
		}
		if err := json.Unmarshal(data, &single); err != nil {
			return nil, err
		}
		if single.Webhook == "" {
			return nil, nil
		}
		return []DiscordReceiver{{Webhook: single.Webhook}}, nil
	}
	if !strings.HasPrefix(trimmed, "[") {
		return nil, nil
	}
	var elems []json.RawMessage
	if err := json.Unmarshal(data, &elems); err != nil {
		return nil, err
	}
	out := make([]DiscordReceiver, 0, len(elems))
	for _, elem := range elems {
		r, err := parseOverrideDiscordElement(elem)
		if err != nil {
			return nil, err
		}
		if r.Webhook != "" {
			out = append(out, r)
		}
	}
	return out, nil
}

func parseOverrideDiscordElement(data json.RawMessage) (DiscordReceiver, error) {
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
