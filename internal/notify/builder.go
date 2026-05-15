package notify

import (
	"strings"

	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
)

// BuildNotifiers returns notifiers for the given monitor. For each channel,
// a non-empty override list fully replaces the global list; otherwise the
// global enabled list is used.
func BuildNotifiers(cfg *config.Config, m *monitor.Monitor) []Notifier {
	out := make([]Notifier, 0, 4)

	for _, t := range telegramTargets(cfg, m) {
		out = append(out, NewTelegram(t.Token, t.ChatID))
	}
	for _, webhook := range discordWebhooks(cfg, m) {
		out = append(out, NewDiscord(webhook))
	}
	return out
}

func telegramTargets(cfg *config.Config, m *monitor.Monitor) []config.TelegramTarget {
	if m != nil && m.NotifyOverride != nil && len(m.NotifyOverride.Telegram) > 0 {
		out := make([]config.TelegramTarget, 0, len(m.NotifyOverride.Telegram))
		for _, t := range m.NotifyOverride.Telegram {
			token := strings.TrimSpace(t.Token)
			chat := strings.TrimSpace(t.ChatID)
			if token != "" && chat != "" {
				out = append(out, config.TelegramTarget{Token: token, ChatID: chat})
			}
		}
		return out
	}
	if cfg.Telegram.Enabled {
		return cfg.Telegram.Targets
	}
	return nil
}

func discordWebhooks(cfg *config.Config, m *monitor.Monitor) []string {
	if m != nil && m.NotifyOverride != nil && len(m.NotifyOverride.Discord) > 0 {
		out := make([]string, 0, len(m.NotifyOverride.Discord))
		for _, w := range m.NotifyOverride.Discord {
			w = strings.TrimSpace(w)
			if w != "" {
				out = append(out, w)
			}
		}
		return out
	}
	if cfg.Discord.Enabled {
		return cfg.Discord.Webhooks
	}
	return nil
}
