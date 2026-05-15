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

// telegramTargets resolves Telegram destinations for one monitor.
//
// Per-channel rule: if the monitor has at least one saved Telegram override
// entry, only those entries are used and all global Telegram receivers are
// ignored. If there is no Telegram override, global settings apply when enabled.
// Discord is resolved separately (see discordWebhooks).
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
		// Override list replaces global even when only one receiver is set.
		return out
	}
	if cfg.Telegram.Enabled {
		return cfg.Telegram.Targets
	}
	return nil
}

// discordWebhooks resolves Discord destinations for one monitor (same
// per-channel replace-vs-global rules as telegramTargets).
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
