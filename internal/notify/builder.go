package notify

import (
	"fmt"
	"strings"

	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
)

// ResolvedReceiver is a notifier with its effective policy for one destination.
type ResolvedReceiver struct {
	Notifier Notifier
	Policy   ResolvedPolicy
	Key      string
}

// BuildReceivers returns notifiers and per-receiver policies for a monitor.
func BuildReceivers(cfg *config.Config, m *monitor.Monitor) []ResolvedReceiver {
	out := make([]ResolvedReceiver, 0, 4)

	for _, t := range telegramTargets(cfg, m) {
		pol := ResolveReceiverPolicy(cfg, t.Policy)
		out = append(out, ResolvedReceiver{
			Notifier: NewTelegram(t.Token, t.ChatID),
			Policy:   pol,
			Key:      fmt.Sprintf("telegram:%s", t.ChatID),
		})
	}
	for _, d := range discordReceivers(cfg, m) {
		pol := ResolveReceiverPolicy(cfg, d.Policy)
		out = append(out, ResolvedReceiver{
			Notifier: NewDiscord(d.Webhook),
			Policy:   pol,
			Key:      discordReceiverKey(d.Webhook),
		})
	}
	return out
}

// BuildNotifiers returns notifiers only (used by tests and simple call sites).
func BuildNotifiers(cfg *config.Config, m *monitor.Monitor) []Notifier {
	recvs := BuildReceivers(cfg, m)
	out := make([]Notifier, len(recvs))
	for i, r := range recvs {
		out[i] = r.Notifier
	}
	return out
}

func discordReceiverKey(webhook string) string {
	w := strings.TrimSpace(webhook)
	if len(w) > 24 {
		return "discord:" + w[len(w)-24:]
	}
	return "discord:" + w
}

func telegramTargets(cfg *config.Config, m *monitor.Monitor) []config.TelegramTarget {
	if m != nil && m.NotifyOverride != nil && len(m.NotifyOverride.Telegram) > 0 {
		return m.NotifyOverride.Telegram
	}
	if cfg != nil && cfg.Telegram.Enabled {
		return cfg.Telegram.Targets
	}
	return nil
}

func discordReceivers(cfg *config.Config, m *monitor.Monitor) []config.DiscordReceiver {
	if m != nil && m.NotifyOverride != nil && len(m.NotifyOverride.Discord) > 0 {
		return m.NotifyOverride.Discord
	}
	if cfg != nil && cfg.Discord.Enabled {
		return cfg.Discord.Webhooks
	}
	return nil
}
