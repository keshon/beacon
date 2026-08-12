package notify

import (
	"net/url"
	"strings"
)

// A receiver's display label.
//
// ResolvedReceiver.Key exists for deduplication and is NOT safe to show: for
// Discord and generic webhooks it is the last thirty-odd characters of the URL,
// and in a Discord webhook that tail IS the token. It does its job as an
// internal identity and must never reach a screen or a stored record.
//
// The label is the opposite: built to be read, and built from the parts that
// carry no secret. A host says where an alert goes, which is what a person
// wants to know; the token says nothing they need and everything an attacker
// does.

// ReceiverLabel returns a display-safe name for a receiver.
func ReceiverLabel(channel, chatID, email, rawURL string) string {
	switch channel {
	case ChannelTelegram:
		if chatID == "" {
			return "telegram"
		}
		return "chat " + chatID
	case ChannelEmail:
		e := strings.ToLower(strings.TrimSpace(email))
		if e == "" {
			return "email"
		}
		return e
	case ChannelDiscord, ChannelWebhook:
		return hostOf(rawURL, channel)
	}
	return channel
}

// hostOf keeps the host and drops everything after it — path, query and
// fragment are exactly where a webhook keeps its secret.
func hostOf(raw, fallback string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fallback
	}
	return u.Host
}
