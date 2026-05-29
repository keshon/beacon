package monitor

import "github.com/keshon/beacon/internal/config"

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
