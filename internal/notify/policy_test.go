package notify

import (
	"testing"

	"github.com/keshon/beacon/internal/config"
)

func TestResolveReceiverPolicy_globalDefaults(t *testing.T) {
	cfg := &config.Config{
		Notifications: config.NotificationsConfig{
			AlertMode: AlertModeRepeat,
			Templates: config.MessageTemplates{
				Down:      "global down",
				Recovered: "global recovered",
			},
		},
	}
	p := ResolveReceiverPolicy(cfg, nil)
	if p.AlertMode != AlertModeRepeat {
		t.Fatalf("alert_mode: got %q", p.AlertMode)
	}
	if p.Templates.Down != "global down" {
		t.Fatalf("down: %q", p.Templates.Down)
	}
}

func TestResolveReceiverPolicy_rowOverridesMode(t *testing.T) {
	cfg := &config.Config{
		Notifications: config.NotificationsConfig{
			AlertMode: AlertModeRepeat,
			Templates: DefaultTemplates(),
		},
	}
	row := &config.ReceiverPolicy{AlertMode: AlertModeOnce}
	p := ResolveReceiverPolicy(cfg, row)
	if p.AlertMode != AlertModeOnce {
		t.Fatalf("want once, got %q", p.AlertMode)
	}
}

func TestResolveReceiverPolicy_perFieldTemplateMerge(t *testing.T) {
	cfg := &config.Config{
		Notifications: config.NotificationsConfig{
			Templates: config.MessageTemplates{
				Down:      "global down",
				Recovered: "global recovered",
			},
		},
	}
	row := &config.ReceiverPolicy{
		Templates: &config.MessageTemplates{Down: "row down"},
	}
	p := ResolveReceiverPolicy(cfg, row)
	if p.Templates.Down != "row down" {
		t.Fatalf("down: %q", p.Templates.Down)
	}
	if p.Templates.Recovered != "global recovered" {
		t.Fatalf("recovered should use global: %q", p.Templates.Recovered)
	}
}

func TestIsCustomTemplates_builtinVsCustom(t *testing.T) {
	def := ResolvedPolicy{Templates: DefaultTemplates()}
	if IsCustomTemplates(def) {
		t.Fatal("builtin should not be custom")
	}
	custom := ResolvedPolicy{
		Templates: config.MessageTemplates{Down: "x", Recovered: DefaultTemplates().Recovered},
	}
	if !IsCustomTemplates(custom) {
		t.Fatal("modified down should be custom")
	}
}

func TestTemplateForStatus_unknownUsesDown(t *testing.T) {
	p := ResolvedPolicy{Templates: config.MessageTemplates{Down: "custom down", Recovered: "custom up"}}
	if got := p.TemplateForStatus("down"); got != "custom down" {
		t.Fatalf("down: got %q", got)
	}
	if got := p.TemplateForStatus("anything"); got != "custom down" {
		t.Fatalf("default should be down template: %q", got)
	}
}
