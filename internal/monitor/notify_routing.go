package monitor

import (
	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor/checks"
)

const (
	NotifyChannelInherit = "inherit"
	NotifyChannelOff     = "off"
	NotifyChannelCustom  = "custom"
)

// NormalizeChannelMode returns a canonical channel override mode.
func NormalizeChannelMode(mode string) string {
	switch mode {
	case NotifyChannelOff:
		return NotifyChannelOff
	case NotifyChannelCustom:
		return NotifyChannelCustom
	default:
		return NotifyChannelInherit
	}
}

// TelegramChannelOverride is per-monitor Telegram routing.
type TelegramChannelOverride struct {
	Mode    string                 `json:"mode,omitempty"`
	Targets []config.TelegramTarget `json:"targets,omitempty"`
}

// DiscordChannelOverride is per-monitor Discord routing.
type DiscordChannelOverride struct {
	Mode    string                  `json:"mode,omitempty"`
	Targets []config.DiscordReceiver `json:"targets,omitempty"`
}

// EmailChannelOverride is per-monitor Email routing.
type EmailChannelOverride struct {
	Mode    string              `json:"mode,omitempty"`
	Targets []config.EmailTarget `json:"targets,omitempty"`
}

// WebhookChannelOverride is per-monitor generic webhook routing.
type WebhookChannelOverride struct {
	Mode    string                    `json:"mode,omitempty"`
	Targets []config.WebhookReceiver `json:"targets,omitempty"`
}

// ChannelMode returns the effective mode for a channel override block.
func ChannelMode(mode string) string {
	return NormalizeChannelMode(mode)
}


// TelegramTarget is a per-monitor Telegram receiver.
type TelegramTarget = config.TelegramTarget

// DiscordReceiver is a per-monitor Discord receiver.
type DiscordReceiver = config.DiscordReceiver

// EmailTarget is a per-monitor email receiver.
type EmailTarget = config.EmailTarget

// WebhookReceiver is a per-monitor generic webhook receiver.
type WebhookReceiver = config.WebhookReceiver

func sanitizeTelegramChannel(in *TelegramChannelOverride) *TelegramChannelOverride {
	if in == nil {
		return nil
	}
	mode := NormalizeChannelMode(in.Mode)
	if mode == NotifyChannelOff {
		return &TelegramChannelOverride{Mode: NotifyChannelOff}
	}
	if mode == NotifyChannelInherit {
		return nil
	}
	targets := config.SanitizeTelegramTargets(in.Targets)
	if len(targets) == 0 {
		return nil
	}
	return &TelegramChannelOverride{Mode: NotifyChannelCustom, Targets: targets}
}

func sanitizeDiscordChannel(in *DiscordChannelOverride) *DiscordChannelOverride {
	if in == nil {
		return nil
	}
	mode := NormalizeChannelMode(in.Mode)
	if mode == NotifyChannelOff {
		return &DiscordChannelOverride{Mode: NotifyChannelOff}
	}
	if mode == NotifyChannelInherit {
		return nil
	}
	targets := config.SanitizeDiscordReceivers(in.Targets)
	if len(targets) == 0 {
		return nil
	}
	return &DiscordChannelOverride{Mode: NotifyChannelCustom, Targets: targets}
}

func sanitizeEmailChannel(in *EmailChannelOverride) *EmailChannelOverride {
	if in == nil {
		return nil
	}
	mode := NormalizeChannelMode(in.Mode)
	if mode == NotifyChannelOff {
		return &EmailChannelOverride{Mode: NotifyChannelOff}
	}
	if mode == NotifyChannelInherit {
		return nil
	}
	targets := config.SanitizeEmailTargets(in.Targets)
	if len(targets) == 0 {
		return nil
	}
	return &EmailChannelOverride{Mode: NotifyChannelCustom, Targets: targets}
}

func sanitizeWebhookChannel(in *WebhookChannelOverride) *WebhookChannelOverride {
	if in == nil {
		return nil
	}
	mode := NormalizeChannelMode(in.Mode)
	if mode == NotifyChannelOff {
		return &WebhookChannelOverride{Mode: NotifyChannelOff}
	}
	if mode == NotifyChannelInherit {
		return nil
	}
	targets := config.SanitizeWebhookReceivers(in.Targets)
	if len(targets) == 0 {
		return nil
	}
	return &WebhookChannelOverride{Mode: NotifyChannelCustom, Targets: targets}
}

// SanitizeNotifyOverride normalizes channel modes and receiver rows.
func SanitizeNotifyOverride(in *NotifyOverride) *NotifyOverride {
	if in == nil {
		return nil
	}
	out := &NotifyOverride{}
	out.Telegram = sanitizeTelegramChannel(in.Telegram)
	out.Discord = sanitizeDiscordChannel(in.Discord)
	out.Email = sanitizeEmailChannel(in.Email)
	out.Webhook = sanitizeWebhookChannel(in.Webhook)
	if out.Telegram == nil && out.Discord == nil && out.Email == nil && out.Webhook == nil {
		return nil
	}
	return out
}

// MergeHTTPOptions applies patch semantics for monitor HTTP options.
func MergeHTTPOptions(existing, incoming *checks.HTTPOptions) *checks.HTTPOptions {
	return checks.MergeHTTPOptions(existing, incoming)
}
