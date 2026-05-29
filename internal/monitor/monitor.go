package monitor

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/keshon/beacon/internal/checks"
	"github.com/keshon/beacon/internal/config"
)

// NotifyOverride holds per-monitor notification routing per channel.
type NotifyOverride struct {
	Telegram  *TelegramChannelOverride  `json:"telegram,omitempty"`
	Discord   *DiscordChannelOverride   `json:"discord,omitempty"`
	Email     *EmailChannelOverride     `json:"email,omitempty"`
	Webhook   *WebhookChannelOverride   `json:"webhook,omitempty"`
	AlertMode string                    `json:"alert_mode,omitempty"` // deprecated
	Templates *config.MessageTemplates  `json:"templates,omitempty"`  // deprecated
}

// UnmarshalJSON accepts tri-state channel blocks and legacy slice shapes.
func (n *NotifyOverride) UnmarshalJSON(data []byte) error {
	*n = NotifyOverride{}
	var raw struct {
		Telegram  json.RawMessage          `json:"telegram"`
		Discord   json.RawMessage          `json:"discord"`
		Email     json.RawMessage          `json:"email"`
		Webhook   json.RawMessage          `json:"webhook"`
		AlertMode string                   `json:"alert_mode"`
		Templates *config.MessageTemplates `json:"templates"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	n.AlertMode = raw.AlertMode
	n.Templates = raw.Templates
	n.Telegram = unmarshalTelegramChannel(raw.Telegram)
	n.Discord = unmarshalDiscordChannel(raw.Discord)
	n.Email = unmarshalEmailChannel(raw.Email)
	n.Webhook = unmarshalWebhookChannel(raw.Webhook)
	MigrateNotifyOverride(n)
	return nil
}

func unmarshalTelegramChannel(data json.RawMessage) *TelegramChannelOverride {
	if len(data) == 0 {
		return nil
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var targets []config.TelegramTarget
		if err := json.Unmarshal(data, &targets); err != nil {
			return nil
		}
		if len(targets) == 0 {
			return &TelegramChannelOverride{Mode: NotifyChannelInherit}
		}
		return &TelegramChannelOverride{Mode: NotifyChannelCustom, Targets: targets}
	}
	var ch TelegramChannelOverride
	if err := json.Unmarshal(data, &ch); err != nil {
		return nil
	}
	if ch.Mode == "" && len(ch.Targets) > 0 {
		ch.Mode = NotifyChannelCustom
	}
	if ch.Mode == "" {
		ch.Mode = NotifyChannelInherit
	}
	return &ch
}

func unmarshalDiscordChannel(data json.RawMessage) *DiscordChannelOverride {
	if len(data) == 0 {
		return nil
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	if strings.HasPrefix(trimmed, "[") {
		targets, err := config.ParseDiscordReceiversJSON(data)
		if err != nil || len(targets) == 0 {
			if len(targets) == 0 {
				return &DiscordChannelOverride{Mode: NotifyChannelInherit}
			}
			return nil
		}
		return &DiscordChannelOverride{Mode: NotifyChannelCustom, Targets: targets}
	}
	var ch DiscordChannelOverride
	if err := json.Unmarshal(data, &ch); err != nil {
		return nil
	}
	if ch.Mode == "" && len(ch.Targets) > 0 {
		ch.Mode = NotifyChannelCustom
	}
	if ch.Mode == "" {
		ch.Mode = NotifyChannelInherit
	}
	return &ch
}

func unmarshalEmailChannel(data json.RawMessage) *EmailChannelOverride {
	if len(data) == 0 {
		return nil
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var targets []config.EmailTarget
		if err := json.Unmarshal(data, &targets); err != nil {
			return nil
		}
		if len(targets) == 0 {
			return &EmailChannelOverride{Mode: NotifyChannelInherit}
		}
		return &EmailChannelOverride{Mode: NotifyChannelCustom, Targets: targets}
	}
	var ch EmailChannelOverride
	if err := json.Unmarshal(data, &ch); err != nil {
		return nil
	}
	if ch.Mode == "" && len(ch.Targets) > 0 {
		ch.Mode = NotifyChannelCustom
	}
	if ch.Mode == "" {
		ch.Mode = NotifyChannelInherit
	}
	return &ch
}

func unmarshalWebhookChannel(data json.RawMessage) *WebhookChannelOverride {
	if len(data) == 0 {
		return nil
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var targets []config.WebhookReceiver
		if err := json.Unmarshal(data, &targets); err != nil {
			return nil
		}
		if len(targets) == 0 {
			return &WebhookChannelOverride{Mode: NotifyChannelInherit}
		}
		return &WebhookChannelOverride{Mode: NotifyChannelCustom, Targets: targets}
	}
	var ch WebhookChannelOverride
	if err := json.Unmarshal(data, &ch); err != nil {
		return nil
	}
	if ch.Mode == "" && len(ch.Targets) > 0 {
		ch.Mode = NotifyChannelCustom
	}
	if ch.Mode == "" {
		ch.Mode = NotifyChannelInherit
	}
	return &ch
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
	Notify         []string        `json:"notify"` // deprecated
	HTTP           *checks.HTTPOptions `json:"http,omitempty"`
	NotifyOverride *NotifyOverride `json:"notify_override,omitempty"`
	OwnerNodeID    string          `json:"owner_node_id,omitempty"`
}
