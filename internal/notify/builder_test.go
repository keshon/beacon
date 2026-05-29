package notify

import (
	"testing"

	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
)

func globalCfg() *config.Config {
	return &config.Config{
		Telegram: config.TelegramConfig{
			Enabled: true,
			Targets: []config.TelegramTarget{
				{Token: "g1", ChatID: "c1"},
				{Token: "g2", ChatID: "c2"},
				{Token: "g3", ChatID: "c3"},
				{Token: "g4", ChatID: "c4"},
				{Token: "g5", ChatID: "c5"},
			},
		},
		Discord: config.DiscordConfig{
			Enabled: true,
			Webhooks: []config.DiscordReceiver{
				{Webhook: "https://global/w1"},
				{Webhook: "https://global/w2"},
				{Webhook: "https://global/w3"},
				{Webhook: "https://global/w4"},
				{Webhook: "https://global/w5"},
			},
		},
	}
}

func tgTokens(targets []config.TelegramTarget) []string {
	out := make([]string, len(targets))
	for i, t := range targets {
		out[i] = t.Token
	}
	return out
}

func customTelegramOverride(targets ...config.TelegramTarget) *monitor.NotifyOverride {
	return &monitor.NotifyOverride{
		Telegram: &monitor.TelegramChannelOverride{
			Mode:    monitor.NotifyChannelCustom,
			Targets: targets,
		},
	}
}

func offTelegramOverride() *monitor.NotifyOverride {
	return &monitor.NotifyOverride{
		Telegram: &monitor.TelegramChannelOverride{Mode: monitor.NotifyChannelOff},
	}
}

func TestTelegramTargets_noOverride_usesAllGlobal(t *testing.T) {
	cfg := globalCfg()
	m := &monitor.Monitor{Name: "m1"}
	got := telegramTargets(cfg, m)
	if len(got) != 5 {
		t.Fatalf("want 5 global telegram targets, got %d", len(got))
	}
}

func TestTelegramTargets_customOverride_ignoresGlobal(t *testing.T) {
	cfg := globalCfg()
	m := &monitor.Monitor{
		NotifyOverride: customTelegramOverride(config.TelegramTarget{Token: "o1", ChatID: "oc1"}),
	}
	got := telegramTargets(cfg, m)
	if len(got) != 1 || got[0].Token != "o1" {
		t.Fatalf("unexpected override: %+v", got)
	}
}

func TestTelegramTargets_offOverride_returnsNil(t *testing.T) {
	cfg := globalCfg()
	m := &monitor.Monitor{NotifyOverride: offTelegramOverride()}
	if got := telegramTargets(cfg, m); len(got) != 0 {
		t.Fatalf("want nil, got %+v", got)
	}
}

func TestDiscordReceivers_customOverride_ignoresGlobal(t *testing.T) {
	cfg := globalCfg()
	m := &monitor.Monitor{
		NotifyOverride: &monitor.NotifyOverride{
			Discord: &monitor.DiscordChannelOverride{
				Mode:    monitor.NotifyChannelCustom,
				Targets: []config.DiscordReceiver{{Webhook: "https://override/w1"}},
			},
		},
	}
	got := discordReceivers(cfg, m)
	if len(got) != 1 || got[0].Webhook != "https://override/w1" {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestChannelsIndependent_discordCustomOnly_usesGlobalTelegram(t *testing.T) {
	cfg := globalCfg()
	m := &monitor.Monitor{
		NotifyOverride: &monitor.NotifyOverride{
			Discord: &monitor.DiscordChannelOverride{
				Mode:    monitor.NotifyChannelCustom,
				Targets: []config.DiscordReceiver{{Webhook: "https://override/w1"}},
			},
		},
	}
	if len(telegramTargets(cfg, m)) != 5 {
		t.Fatal("telegram should inherit global")
	}
	if len(discordReceivers(cfg, m)) != 1 {
		t.Fatal("discord should use custom")
	}
}

func TestTelegramTargets_offDiscordCustom_emailOnlyScenario(t *testing.T) {
	cfg := globalCfg()
	cfg.Email.Enabled = true
	cfg.Email.Targets = []config.EmailTarget{{To: "ops@example.com"}}
	m := &monitor.Monitor{
		NotifyOverride: &monitor.NotifyOverride{
			Telegram: &monitor.TelegramChannelOverride{Mode: monitor.NotifyChannelOff},
			Discord:  &monitor.DiscordChannelOverride{Mode: monitor.NotifyChannelOff},
			Email: &monitor.EmailChannelOverride{
				Mode:    monitor.NotifyChannelCustom,
				Targets: []config.EmailTarget{{To: "only@example.com"}},
			},
		},
	}
	if len(telegramTargets(cfg, m)) != 0 || len(discordReceivers(cfg, m)) != 0 {
		t.Fatal("tg/dc should be off")
	}
	if len(emailTargets(cfg, m)) != 1 || emailTargets(cfg, m)[0].To != "only@example.com" {
		t.Fatal("email custom expected")
	}
}

func TestBuildReceivers_distinctPolicies(t *testing.T) {
	cfg := globalCfg()
	cfg.Discord.Enabled = false
	cfg.Notifications.AlertMode = AlertModeRepeat
	m := &monitor.Monitor{
		NotifyOverride: customTelegramOverride(
			config.TelegramTarget{Token: "o1", ChatID: "c1", Policy: &config.ReceiverPolicy{AlertMode: AlertModeOnce}},
			config.TelegramTarget{Token: "o2", ChatID: "c2"},
		),
	}
	recvs := BuildReceivers(cfg, m)
	if len(recvs) != 2 {
		t.Fatalf("want 2, got %d", len(recvs))
	}
	if recvs[0].Policy.AlertMode != AlertModeOnce {
		t.Fatalf("first: %q", recvs[0].Policy.AlertMode)
	}
}

func TestTelegramTargets_oneOverride_doesNotIncludeGlobalTokens(t *testing.T) {
	cfg := globalCfg()
	m := &monitor.Monitor{
		NotifyOverride: customTelegramOverride(config.TelegramTarget{Token: "only", ChatID: "x"}),
	}
	for _, tok := range tgTokens(telegramTargets(cfg, m)) {
		if tok == "g1" || tok == "g5" {
			t.Fatalf("global token leaked: %v", tok)
		}
	}
}
