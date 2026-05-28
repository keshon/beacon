package monitor

import (
	"encoding/json"
	"testing"

	"github.com/keshon/beacon/internal/config"
)

func TestMigrateNotifyOverride_legacyFieldsToRows(t *testing.T) {
	n := &NotifyOverride{
		Telegram: []TelegramTarget{{Token: "t", ChatID: "c"}},
		Discord:  []DiscordReceiver{{Webhook: "https://example/hook"}},
		AlertMode: "once",
		Templates: &config.MessageTemplates{Down: "legacy down"},
	}
	MigrateNotifyOverride(n)
	if n.AlertMode != "" || n.Templates != nil {
		t.Fatal("legacy top-level fields should be cleared")
	}
	if n.Telegram[0].Policy == nil || n.Telegram[0].Policy.AlertMode != "once" {
		t.Fatalf("telegram policy: %+v", n.Telegram[0].Policy)
	}
	if n.Discord[0].Policy == nil || n.Discord[0].Policy.Templates == nil {
		t.Fatal("discord should inherit legacy templates")
	}
	if n.Discord[0].Policy.Templates.Down != "legacy down" {
		t.Fatalf("discord down: %q", n.Discord[0].Policy.Templates.Down)
	}
}

func TestNotifyOverride_unmarshalDiscordStrings(t *testing.T) {
	var n NotifyOverride
	if err := json.Unmarshal([]byte(`{"discord":["https://a","https://b"]}`), &n); err != nil {
		t.Fatal(err)
	}
	if len(n.Discord) != 2 || n.Discord[0].Webhook != "https://a" {
		t.Fatalf("got %+v", n.Discord)
	}
}
