package config

import "testing"

func TestResolveTelegramTestCredentials(t *testing.T) {
	cfg := &Config{
		Telegram: TelegramConfig{
			Targets: []TelegramTarget{{Token: "tok", ChatID: "123"}},
		},
	}
	token, chat, ok := cfg.ResolveTelegramTestCredentials("", "123")
	if !ok || token != "tok" || chat != "123" {
		t.Fatalf("expected stored credentials, got %q %q %v", token, chat, ok)
	}
	if _, _, ok := cfg.ResolveTelegramTestCredentials("other", "123"); ok {
		t.Fatal("foreign token should be rejected")
	}
}

func TestResolveDiscordTestWebhook(t *testing.T) {
	cfg := &Config{
		Discord: DiscordConfig{
			Webhooks: []DiscordReceiver{{Webhook: "https://discord.test/hook"}},
		},
	}
	allowed, ok := cfg.ResolveDiscordTestWebhook("https://discord.test/hook")
	if !ok || allowed != "https://discord.test/hook" {
		t.Fatalf("expected allowed webhook, got %q %v", allowed, ok)
	}
	if _, ok := cfg.ResolveDiscordTestWebhook("https://evil.test/hook"); ok {
		t.Fatal("unknown webhook should be rejected")
	}
}
