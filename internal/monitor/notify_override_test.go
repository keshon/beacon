package monitor

import (
	"encoding/json"
	"testing"

	"github.com/keshon/beacon/internal/config"
)

func TestMigrateNotifyOverride_legacyFieldsToRows(t *testing.T) {
	n := &NotifyOverride{
		Telegram: &TelegramChannelOverride{
			Mode:    NotifyChannelCustom,
			Targets: []TelegramTarget{{Token: "t", ChatID: "c"}},
		},
		Discord: &DiscordChannelOverride{
			Mode:    NotifyChannelCustom,
			Targets: []DiscordReceiver{{Webhook: "https://example/hook"}},
		},
		AlertMode: "once",
		Templates: &config.MessageTemplates{Down: "legacy down"},
	}
	MigrateNotifyOverride(n)
	if n.AlertMode != "" || n.Templates != nil {
		t.Fatal("legacy top-level fields should be cleared")
	}
	if n.Telegram.Targets[0].Policy == nil || n.Telegram.Targets[0].Policy.AlertMode != "once" {
		t.Fatalf("telegram policy: %+v", n.Telegram.Targets[0].Policy)
	}
}

func TestNotifyOverride_unmarshalDiscordStrings(t *testing.T) {
	var n NotifyOverride
	if err := json.Unmarshal([]byte(`{"discord":["https://a","https://b"]}`), &n); err != nil {
		t.Fatal(err)
	}
	if n.Discord == nil || n.Discord.Mode != NotifyChannelCustom || len(n.Discord.Targets) != 2 {
		t.Fatalf("got %+v", n.Discord)
	}
}

func TestNotifyOverride_unmarshalTelegramOff(t *testing.T) {
	var n NotifyOverride
	if err := json.Unmarshal([]byte(`{"telegram":{"mode":"off"}}`), &n); err != nil {
		t.Fatal(err)
	}
	if n.Telegram == nil || n.Telegram.Mode != NotifyChannelOff {
		t.Fatalf("got %+v", n.Telegram)
	}
}
