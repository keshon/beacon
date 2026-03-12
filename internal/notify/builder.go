package notify

import (
	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
)

// BuildNotifiers returns notifiers for the given monitor, merging global config with per-monitor overrides.
func BuildNotifiers(cfg *config.Config, m *monitor.Monitor) []Notifier {
	var out []Notifier

	// Telegram: use override if set, else global
	useTelegram := false
	token, chatID := "", ""
	if m.NotifyOverride != nil && m.NotifyOverride.Telegram != nil &&
		m.NotifyOverride.Telegram.Token != "" && m.NotifyOverride.Telegram.ChatID != "" {
		useTelegram = true
		token = m.NotifyOverride.Telegram.Token
		chatID = m.NotifyOverride.Telegram.ChatID
	} else if cfg.Telegram.Enabled && cfg.Telegram.Token != "" && cfg.Telegram.ChatID != "" {
		useTelegram = true
		token = cfg.Telegram.Token
		chatID = cfg.Telegram.ChatID
	}
	if useTelegram {
		out = append(out, NewTelegram(token, chatID))
	}

	// Discord: use override if set, else global
	useDiscord := false
	webhook := ""
	if m.NotifyOverride != nil && m.NotifyOverride.Discord != nil && m.NotifyOverride.Discord.Webhook != "" {
		useDiscord = true
		webhook = m.NotifyOverride.Discord.Webhook
	} else if cfg.Discord.Enabled && cfg.Discord.Webhook != "" {
		useDiscord = true
		webhook = cfg.Discord.Webhook
	}
	if useDiscord {
		out = append(out, NewDiscord(webhook))
	}

	return out
}
