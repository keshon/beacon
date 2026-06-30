package config

import "testing"

func TestMaskSecret(t *testing.T) {
	if got := MaskSecret(""); got != "" {
		t.Fatalf("empty: %q", got)
	}
	if got := MaskSecret("short"); got != "•••••" {
		t.Fatalf("short: %q", got)
	}
	got := MaskSecret("12345678901234567890")
	if got[:4] != "1234" || got[len(got)-4:] != "7890" {
		t.Fatalf("long mask: %q", got)
	}
	if !stringsAllBullets(got[4 : len(got)-4]) {
		t.Fatalf("middle should be bullets: %q", got)
	}
}

func stringsAllBullets(s string) bool {
	for _, r := range s {
		if r != '•' {
			return false
		}
	}
	return len(s) > 0
}

func TestSecretUnchanged(t *testing.T) {
	stored := "1234567890:ABCdefGHIjklMNOpqrs"
	masked := MaskSecret(stored)
	if !SecretUnchanged("", stored) {
		t.Fatal("empty incoming keeps stored")
	}
	if !SecretUnchanged(masked, stored) {
		t.Fatal("masked incoming keeps stored")
	}
	if !SecretUnchanged(stored, stored) {
		t.Fatal("exact match")
	}
	if SecretUnchanged("totally-different", stored) {
		t.Fatal("different value should not match")
	}
}

func TestToPublic_masksTelegramAndDiscordSecrets(t *testing.T) {
	cfg := &Config{
		Telegram: TelegramConfig{
			Enabled: true,
			Targets: []TelegramTarget{{Token: "1234567890:AAExampleToken", ChatID: "99"}},
		},
		Discord: DiscordConfig{
			Enabled:  true,
			Webhooks: []DiscordReceiver{{Webhook: "https://discord.com/api/webhooks/1234567890/abcdefghijklmnop"}},
		},
	}
	pub := cfg.ToPublic()
	if pub.Telegram.Targets[0].Token == cfg.Telegram.Targets[0].Token {
		t.Fatal("telegram token must be masked in public config")
	}
	if pub.Telegram.Targets[0].Token == "" {
		t.Fatal("telegram token preview expected")
	}
	if pub.Discord.Webhooks[0].Webhook == cfg.Discord.Webhooks[0].Webhook {
		t.Fatal("discord webhook must be masked")
	}
	if !pub.Secrets.TelegramTokens[0] || !pub.Secrets.DiscordWebhooks[0] {
		t.Fatal("secrets flags expected")
	}
}

func TestMergeTelegramTargets_keepsTokenWhenMasked(t *testing.T) {
	existing := []TelegramTarget{{Token: "1234567890:AARealSecretToken", ChatID: "42"}}
	incoming := []TelegramTarget{{Token: MaskSecret(existing[0].Token), ChatID: "42"}}
	got := mergeTelegramTargets(existing, incoming)
	if len(got) != 1 || got[0].Token != existing[0].Token {
		t.Fatalf("expected stored token preserved, got %+v", got)
	}
}

func TestToSettings_returnsFullSecrets(t *testing.T) {
	cfg := &Config{
		Telegram: TelegramConfig{
			Targets: []TelegramTarget{{Token: "full-telegram-token", ChatID: "1"}},
		},
		Discord: DiscordConfig{
			Webhooks: []DiscordReceiver{{Webhook: "https://discord.com/api/webhooks/full"}},
		},
		Network: NetworkConfig{SyncToken: "sync-secret"},
	}
	settings := cfg.ToSettings()
	if settings.Telegram.Targets[0].Token != "full-telegram-token" {
		t.Fatalf("settings token: %q", settings.Telegram.Targets[0].Token)
	}
	if settings.Discord.Webhooks[0].Webhook != "https://discord.com/api/webhooks/full" {
		t.Fatalf("settings webhook: %q", settings.Discord.Webhooks[0].Webhook)
	}
	if settings.Network.SyncToken != "sync-secret" {
		t.Fatalf("settings sync token: %q", settings.Network.SyncToken)
	}
	pub := cfg.ToPublic()
	if pub.Telegram.Targets[0].Token == "full-telegram-token" {
		t.Fatal("ToPublic must still mask telegram token")
	}
}

func TestMergeDiscordWebhooks_keepsWebhookWhenMasked(t *testing.T) {
	existing := []DiscordReceiver{{Webhook: "https://discord.com/api/webhooks/1/abcdefghijklmnop"}}
	incoming := []DiscordReceiver{{Webhook: MaskSecret(existing[0].Webhook)}}
	got := mergeDiscordWebhooks(existing, incoming)
	if len(got) != 1 || got[0].Webhook != existing[0].Webhook {
		t.Fatalf("expected stored webhook preserved, got %+v", got)
	}
}
