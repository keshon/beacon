package notify

import (
	"testing"
	"time"

	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
)

func TestRecommendedMinInterval_emailOnly_usesProbeFloor(t *testing.T) {
	cfg := &config.Config{
		Email: config.EmailConfig{
			Enabled: true,
			Targets: []config.EmailTarget{{To: "a@example.com"}},
		},
	}
	m := &monitor.Monitor{Name: "m1"}
	got := RecommendedMinInterval(cfg, m)
	if got != minIntervalProbe {
		t.Fatalf("email-only want probe floor %v, got %v", minIntervalProbe, got)
	}
}

func TestRecommendedMinInterval_repeatTelegram(t *testing.T) {
	cfg := &config.Config{
		Telegram: config.TelegramConfig{
			Enabled: true,
			Targets: []config.TelegramTarget{
				{Token: "t", ChatID: "c", Policy: &config.ReceiverPolicy{AlertMode: AlertModeRepeat}},
			},
		},
	}
	m := &monitor.Monitor{Name: "m1"}
	got := RecommendedMinInterval(cfg, m)
	if got != minIntervalTelegram {
		t.Fatalf("want telegram floor %v, got %v", minIntervalTelegram, got)
	}
}

func TestRecommendedMinInterval_onceMode_usesProbeFloor(t *testing.T) {
	cfg := &config.Config{
		Discord: config.DiscordConfig{
			Enabled: true,
			Webhooks: []config.DiscordReceiver{
				{Webhook: "https://example/h", Policy: &config.ReceiverPolicy{AlertMode: AlertModeOnce}},
			},
		},
	}
	m := &monitor.Monitor{Name: "m1"}
	got := RecommendedMinInterval(cfg, m)
	if got != minIntervalProbe {
		t.Fatalf("once mode want probe floor %v, got %v", minIntervalProbe, got)
	}
}

func TestIntervalWarnings_belowRecommended(t *testing.T) {
	cfg := &config.Config{
		Webhook: config.WebhookConfig{
			Enabled: true,
			Webhooks: []config.WebhookReceiver{
				{URL: "https://example/h", Policy: &config.ReceiverPolicy{AlertMode: AlertModeRepeat}},
			},
		},
	}
	m := &monitor.Monitor{Name: "m1", Interval: 10 * time.Second}
	warn := IntervalWarnings(cfg, m)
	if len(warn) != 1 {
		t.Fatalf("want 1 warning, got %v", warn)
	}
}

func TestIntervalWarnings_okInterval(t *testing.T) {
	cfg := &config.Config{
		Telegram: config.TelegramConfig{
			Enabled: true,
			Targets: []config.TelegramTarget{
				{Token: "t", ChatID: "c", Policy: &config.ReceiverPolicy{AlertMode: AlertModeRepeat}},
			},
		},
	}
	m := &monitor.Monitor{Name: "m1", Interval: 30 * time.Second}
	if len(IntervalWarnings(cfg, m)) != 0 {
		t.Fatal("expected no warnings")
	}
}
