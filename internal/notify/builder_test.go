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

func TestTelegramTargets_noOverride_usesAllGlobal(t *testing.T) {
	cfg := globalCfg()
	m := &monitor.Monitor{Name: "m1"}
	got := telegramTargets(cfg, m)
	if len(got) != 5 {
		t.Fatalf("want 5 global telegram targets, got %d", len(got))
	}
	if got[0].Token != "g1" || got[4].Token != "g5" {
		t.Fatalf("unexpected global targets: %+v", got)
	}
}

func TestTelegramTargets_oneOverride_ignoresGlobal(t *testing.T) {
	cfg := globalCfg()
	m := &monitor.Monitor{
		NotifyOverride: &monitor.NotifyOverride{
			Telegram: []monitor.TelegramTarget{
				{Token: "o1", ChatID: "oc1"},
			},
		},
	}
	got := telegramTargets(cfg, m)
	if len(got) != 1 {
		t.Fatalf("want 1 override target, got %d: %+v", len(got), got)
	}
	if got[0].Token != "o1" {
		t.Fatalf("want override token o1, got %q", got[0].Token)
	}
}

func TestDiscordReceivers_oneOverride_ignoresGlobal(t *testing.T) {
	cfg := globalCfg()
	m := &monitor.Monitor{
		NotifyOverride: &monitor.NotifyOverride{
			Discord: []monitor.DiscordReceiver{{Webhook: "https://override/w1"}},
		},
	}
	got := discordReceivers(cfg, m)
	if len(got) != 1 {
		t.Fatalf("want 1 override webhook, got %d: %+v", len(got), got)
	}
	if got[0].Webhook != "https://override/w1" {
		t.Fatalf("unexpected webhook %q", got[0].Webhook)
	}
}

func TestChannelsIndependent_discordOverrideOnly_usesGlobalTelegram(t *testing.T) {
	cfg := globalCfg()
	m := &monitor.Monitor{
		NotifyOverride: &monitor.NotifyOverride{
			Discord: []monitor.DiscordReceiver{{Webhook: "https://override/w1"}},
		},
	}
	tg := telegramTargets(cfg, m)
	dc := discordReceivers(cfg, m)
	if len(tg) != 5 {
		t.Fatalf("telegram should use all 5 global, got %d", len(tg))
	}
	if len(dc) != 1 {
		t.Fatalf("discord should use 1 override, got %d", len(dc))
	}
}

func TestChannelsIndependent_telegramOverrideOnly_usesGlobalDiscord(t *testing.T) {
	cfg := globalCfg()
	m := &monitor.Monitor{
		NotifyOverride: &monitor.NotifyOverride{
			Telegram: []monitor.TelegramTarget{
				{Token: "o1", ChatID: "oc1"},
			},
		},
	}
	tg := telegramTargets(cfg, m)
	dc := discordReceivers(cfg, m)
	if len(tg) != 1 {
		t.Fatalf("telegram should use 1 override, got %d", len(tg))
	}
	if len(dc) != 5 {
		t.Fatalf("discord should use all 5 global, got %d", len(dc))
	}
}

func TestBuildReceivers_countsMatchTargets(t *testing.T) {
	cfg := globalCfg()
	m := &monitor.Monitor{
		NotifyOverride: &monitor.NotifyOverride{
			Telegram: []monitor.TelegramTarget{
				{Token: "o1", ChatID: "oc1"},
				{Token: "o2", ChatID: "oc2"},
			},
			Discord: []monitor.DiscordReceiver{{Webhook: "https://override/w1"}},
		},
	}
	recvs := BuildReceivers(cfg, m)
	if len(recvs) != 3 {
		t.Fatalf("want 3 receivers (2 tg + 1 dc), got %d", len(recvs))
	}
}

func TestBuildReceivers_distinctPolicies(t *testing.T) {
	cfg := globalCfg()
	cfg.Discord.Enabled = false
	cfg.Notifications.AlertMode = AlertModeRepeat
	m := &monitor.Monitor{
		NotifyOverride: &monitor.NotifyOverride{
			Telegram: []monitor.TelegramTarget{
				{
					Token:  "o1",
					ChatID: "c1",
					Policy: &config.ReceiverPolicy{AlertMode: AlertModeOnce},
				},
				{Token: "o2", ChatID: "c2"},
			},
		},
	}
	recvs := BuildReceivers(cfg, m)
	if len(recvs) != 2 {
		t.Fatalf("want 2, got %d", len(recvs))
	}
	if recvs[0].Policy.AlertMode != AlertModeOnce {
		t.Fatalf("first: %q", recvs[0].Policy.AlertMode)
	}
	if recvs[1].Policy.AlertMode != AlertModeRepeat {
		t.Fatalf("second should inherit global repeat: %q", recvs[1].Policy.AlertMode)
	}
}

func TestTelegramTargets_emptyOverrideSlice_usesGlobal(t *testing.T) {
	cfg := globalCfg()
	m := &monitor.Monitor{
		NotifyOverride: &monitor.NotifyOverride{
			Telegram: []monitor.TelegramTarget{},
			Discord:  []monitor.DiscordReceiver{{Webhook: "https://override/w1"}},
		},
	}
	got := telegramTargets(cfg, m)
	if len(got) != 5 {
		t.Fatalf("empty telegram override slice should fall back to global, got %d", len(got))
	}
}

func TestTelegramTargets_globalDisabled_noOverride_returnsNil(t *testing.T) {
	cfg := globalCfg()
	cfg.Telegram.Enabled = false
	m := &monitor.Monitor{Name: "m1"}
	if got := telegramTargets(cfg, m); len(got) != 0 {
		t.Fatalf("want no telegram when global disabled, got %+v", got)
	}
}

func TestTelegramTargets_overrideWorksWhenGlobalDisabled(t *testing.T) {
	cfg := globalCfg()
	cfg.Telegram.Enabled = false
	m := &monitor.Monitor{
		NotifyOverride: &monitor.NotifyOverride{
			Telegram: []monitor.TelegramTarget{{Token: "o1", ChatID: "c1"}},
		},
	}
	got := telegramTargets(cfg, m)
	if len(got) != 1 || got[0].Token != "o1" {
		t.Fatalf("override should work when global disabled, got %+v", got)
	}
}

// Guard against accidental merge of global + override lists.
func TestTelegramTargets_oneOverride_doesNotIncludeGlobalTokens(t *testing.T) {
	cfg := globalCfg()
	m := &monitor.Monitor{
		NotifyOverride: &monitor.NotifyOverride{
			Telegram: []monitor.TelegramTarget{{Token: "only", ChatID: "x"}},
		},
	}
	tokens := tgTokens(telegramTargets(cfg, m))
	for _, tok := range tokens {
		if tok == "g1" || tok == "g5" {
			t.Fatalf("global token leaked into override result: %v", tokens)
		}
	}
}
