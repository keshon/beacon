package notify

import (
	"fmt"
	"strings"

	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
)

const (
	ChannelTelegram = "telegram"
	ChannelDiscord  = "discord"
	ChannelEmail    = "email"
	ChannelWebhook  = "webhook"
)

// ResolvedReceiver is a notifier with its effective policy for one destination.
type ResolvedReceiver struct {
	Notifier Notifier
	Policy   ResolvedPolicy
	Key      string
	Channel  string
}

// BuildReceivers returns notifiers and per-receiver policies for a monitor.
func BuildReceivers(cfg *config.Config, m *monitor.Monitor) []ResolvedReceiver {
	out := make([]ResolvedReceiver, 0, 8)

	for _, t := range telegramTargets(cfg, m) {
		pol := ResolveReceiverPolicy(cfg, t.Policy)
		out = append(out, ResolvedReceiver{
			Notifier: NewTelegram(t.Token, t.ChatID),
			Policy:   pol,
			Key:      fmt.Sprintf("telegram:%s", t.ChatID),
			Channel:  ChannelTelegram,
		})
	}
	for _, d := range discordReceivers(cfg, m) {
		pol := ResolveReceiverPolicy(cfg, d.Policy)
		out = append(out, ResolvedReceiver{
			Notifier: NewDiscord(d.Webhook),
			Policy:   pol,
			Key:      discordReceiverKey(d.Webhook),
			Channel:  ChannelDiscord,
		})
	}
	for _, e := range emailTargets(cfg, m) {
		smtp := cfg.EffectiveSMTP(e)
		pol := ResolveReceiverPolicy(cfg, e.Policy)
		out = append(out, ResolvedReceiver{
			Notifier: NewEmail(smtp, e.To),
			Policy:   pol,
			Key:      fmt.Sprintf("email:%s", strings.ToLower(strings.TrimSpace(e.To))),
			Channel:  ChannelEmail,
		})
	}
	for _, w := range webhookReceivers(cfg, m) {
		pol := ResolveReceiverPolicy(cfg, w.Policy)
		out = append(out, ResolvedReceiver{
			Notifier: NewWebhook(w.URL, w.Headers),
			Policy:   pol,
			Key:      webhookReceiverKey(w.URL),
			Channel:  ChannelWebhook,
		})
	}
	return out
}

func webhookReceiverKey(url string) string {
	u := strings.TrimSpace(url)
	if len(u) > 32 {
		return "webhook:" + u[len(u)-32:]
	}
	return "webhook:" + u
}

func telegramTargets(cfg *config.Config, m *monitor.Monitor) []config.TelegramTarget {
	var ov *monitor.TelegramChannelOverride
	if m != nil && m.NotifyOverride != nil {
		ov = m.NotifyOverride.Telegram
	}
	if ov != nil {
		switch monitor.NormalizeChannelMode(ov.Mode) {
		case monitor.NotifyChannelOff:
			return nil
		case monitor.NotifyChannelCustom:
			return ov.Targets
		}
	}
	if cfg != nil && cfg.Telegram.Enabled {
		return cfg.Telegram.Targets
	}
	return nil
}

func discordReceivers(cfg *config.Config, m *monitor.Monitor) []config.DiscordReceiver {
	var ov *monitor.DiscordChannelOverride
	if m != nil && m.NotifyOverride != nil {
		ov = m.NotifyOverride.Discord
	}
	if ov != nil {
		switch monitor.NormalizeChannelMode(ov.Mode) {
		case monitor.NotifyChannelOff:
			return nil
		case monitor.NotifyChannelCustom:
			return ov.Targets
		}
	}
	if cfg != nil && cfg.Discord.Enabled {
		return cfg.Discord.Webhooks
	}
	return nil
}

func emailTargets(cfg *config.Config, m *monitor.Monitor) []config.EmailTarget {
	var ov *monitor.EmailChannelOverride
	if m != nil && m.NotifyOverride != nil {
		ov = m.NotifyOverride.Email
	}
	if ov != nil {
		switch monitor.NormalizeChannelMode(ov.Mode) {
		case monitor.NotifyChannelOff:
			return nil
		case monitor.NotifyChannelCustom:
			return ov.Targets
		}
	}
	if cfg != nil && cfg.Email.Enabled {
		return cfg.Email.Targets
	}
	return nil
}

func webhookReceivers(cfg *config.Config, m *monitor.Monitor) []config.WebhookReceiver {
	var ov *monitor.WebhookChannelOverride
	if m != nil && m.NotifyOverride != nil {
		ov = m.NotifyOverride.Webhook
	}
	if ov != nil {
		switch monitor.NormalizeChannelMode(ov.Mode) {
		case monitor.NotifyChannelOff:
			return nil
		case monitor.NotifyChannelCustom:
			return ov.Targets
		}
	}
	if cfg != nil && cfg.Webhook.Enabled {
		return cfg.Webhook.Webhooks
	}
	return nil
}

func discordReceiverKey(webhook string) string {
	w := strings.TrimSpace(webhook)
	if len(w) > 24 {
		return "discord:" + w[len(w)-24:]
	}
	return "discord:" + w
}
