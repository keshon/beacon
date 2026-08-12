package config

import (
	"encoding/json"
	"testing"
)

// The whole point: a screen saving its own section must not touch anyone
// else's. Before sections existed, this test would have failed by wiping the
// Telegram channel that the patch never mentioned.
func TestPartialPatchLeavesOtherSectionsAlone(t *testing.T) {
	existing := Default()
	existing.Listen = ":9000"
	existing.Telegram = TelegramConfig{
		Enabled: true,
		Targets: []TelegramTarget{{Token: "secret-token", ChatID: "-100500"}},
	}
	existing.Network.Enabled = true
	existing.Network.SyncToken = "sync-secret"

	// A patch from the notifications screen: email only.
	body := []byte(`{"email":{"enabled":true,"smtp":{"host":"smtp.example.com","port":587,"from":"a@b.c"},"targets":[{"to":"ops@example.com"}]}}`)
	sec, err := ParseSections(body)
	if err != nil {
		t.Fatal(err)
	}
	incoming := Config{}
	if err := json.Unmarshal(body, &incoming); err != nil {
		t.Fatal(err)
	}

	ApplyNonSecret(existing, &incoming, sec)
	if err := MergeSecrets(existing, &incoming, sec); err != nil {
		t.Fatal(err)
	}

	if existing.Listen != ":9000" {
		t.Fatalf("listen was wiped by a patch that never mentioned it: %q", existing.Listen)
	}
	if !existing.Telegram.Enabled || len(existing.Telegram.Targets) != 1 {
		t.Fatalf("telegram was wiped: %#v", existing.Telegram)
	}
	if existing.Telegram.Targets[0].Token != "secret-token" {
		t.Fatal("the telegram token was lost")
	}
	if !existing.Network.Enabled || existing.Network.SyncToken != "sync-secret" {
		t.Fatalf("network was wiped: %#v", existing.Network)
	}
	if !existing.Email.Enabled || len(existing.Email.Targets) != 1 {
		t.Fatalf("the section that WAS in the patch did not apply: %#v", existing.Email)
	}
}

// A patch that mentions a section wins over what is stored, including turning
// something off — otherwise a channel could never be disabled.
func TestMentionedSectionCanTurnThingsOff(t *testing.T) {
	existing := Default()
	existing.Telegram = TelegramConfig{Enabled: true, Targets: []TelegramTarget{{Token: "t", ChatID: "1"}}}

	body := []byte(`{"telegram":{"enabled":false,"targets":[]}}`)
	sec, _ := ParseSections(body)
	var incoming Config
	if err := json.Unmarshal(body, &incoming); err != nil {
		t.Fatal(err)
	}
	ApplyNonSecret(existing, &incoming, sec)
	if err := MergeSecrets(existing, &incoming, sec); err != nil {
		t.Fatal(err)
	}
	if existing.Telegram.Enabled {
		t.Fatal("a channel could not be switched off")
	}
}

// Old clients send the whole config and no section information. They must keep
// behaving exactly as before.
func TestAllSectionsTouchesEverything(t *testing.T) {
	existing := Default()
	existing.Listen = ":9000"

	var incoming Config
	incoming.Listen = ":7000"
	ApplyNonSecret(existing, &incoming, AllSections())
	if existing.Listen != ":7000" {
		t.Fatalf("a whole-config apply stopped applying: %q", existing.Listen)
	}
}
