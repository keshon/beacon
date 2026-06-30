package notify

import (
	"strings"
	"testing"
	"time"
)

func TestAlertText_usesBodyWhenSet(t *testing.T) {
	got := AlertText(Alert{Body: "custom body"})
	if got != "custom body" {
		t.Fatalf("got %q", got)
	}
}

func TestAlertText_fallsBackToDefaultTemplate(t *testing.T) {
	a := Alert{
		MonitorName: "api.example.com",
		Status:      "down",
		Message:     "Error: timeout",
		Time:        time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC),
	}
	got := AlertText(a)
	if !strings.Contains(got, "DOWN") {
		t.Fatalf("expected DOWN header, got %q", got)
	}
	if !strings.Contains(got, "api.example.com") {
		t.Fatalf("expected monitor name at end, got %q", got)
	}
	if strings.Contains(got, "Service DOWN") {
		t.Fatalf("legacy format leaked: %q", got)
	}
}
