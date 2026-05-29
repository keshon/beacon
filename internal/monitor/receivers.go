package monitor

import (
	"github.com/keshon/beacon/internal/config"
)

// TelegramTarget is a per-monitor Telegram receiver (same shape as config.TelegramTarget).
type TelegramTarget = config.TelegramTarget

// DiscordReceiver is a per-monitor Discord receiver (same shape as config.DiscordReceiver).
type DiscordReceiver = config.DiscordReceiver

// SanitizeNotifyOverride drops incomplete rows, trims whitespace, and caps each channel.
// Returns nil when the override is empty.
func SanitizeNotifyOverride(in *NotifyOverride) *NotifyOverride {
	if in == nil {
		return nil
	}
	MigrateNotifyOverride(in)
	out := &NotifyOverride{}
	out.Telegram = config.SanitizeTelegramTargets(in.Telegram)
	out.Discord = config.SanitizeDiscordReceivers(in.Discord)
	if len(out.Telegram) == 0 && len(out.Discord) == 0 {
		return nil
	}
	return out
}
