package notify

import (
	"strings"
	"testing"
)

// The receiver key is not display-safe and the label is. This test exists
// because the difference is a secret: for a Discord webhook the key's tail IS
// the token, and a label that leaked it would leak it into stored records and
// onto a screen at once.
func TestLabelNeverCarriesTheWebhookSecret(t *testing.T) {
	const secret = "aVeryLongDiscordTokenThatMustNeverBeShown"
	webhook := "https://discord.com/api/webhooks/123456789/" + secret

	label := ReceiverLabel(ChannelDiscord, "", "", webhook)
	if strings.Contains(label, secret) {
		t.Fatalf("the label leaked the token: %q", label)
	}
	if strings.Contains(label, "/") {
		t.Fatalf("the label kept a path, which is where the secret lives: %q", label)
	}
	if label != "discord.com" {
		t.Fatalf("want the host, got %q", label)
	}

	// The key, by contrast, is expected to carry the tail — that is what makes
	// it a good deduplication identity and a bad label.
	if !strings.Contains(discordReceiverKey(webhook), secret[len(secret)-10:]) {
		t.Fatal("the key stopped identifying the receiver")
	}
}

func TestLabelsForTheOtherChannels(t *testing.T) {
	cases := []struct {
		channel, chatID, email, url, want string
	}{
		{ChannelTelegram, "-100123", "", "", "chat -100123"},
		{ChannelTelegram, "", "", "", "telegram"},
		{ChannelEmail, "", "  OPS@Example.com ", "", "ops@example.com"},
		{ChannelEmail, "", "", "", "email"},
		{ChannelWebhook, "", "", "https://hooks.example.com/x/y?token=abc", "hooks.example.com"},
		{ChannelWebhook, "", "", "not a url at all", "webhook"},
	}
	for _, c := range cases {
		if got := ReceiverLabel(c.channel, c.chatID, c.email, c.url); got != c.want {
			t.Fatalf("%s: want %q, got %q", c.channel, c.want, got)
		}
	}
}
