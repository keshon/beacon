package notify

import (
	"strings"
	"testing"
	"time"

	"github.com/keshon/beacon/internal/checks"
	"github.com/keshon/beacon/internal/monitor"
)

func TestRenderTemplate_placeholders(t *testing.T) {
	m := &monitor.Monitor{Name: "API", Target: "https://x.com", Type: "http"}
	st := &monitor.MonitorState{FailCount: 2}
	res := checks.CheckResult{
		Success:    false,
		Error:      "connection refused",
		StatusCode: 503,
		Latency:    time.Millisecond * 42,
		Time:       time.Date(2026, 1, 15, 12, 30, 0, 0, time.UTC),
	}
	ctx := NewTemplateContext(m, st, res, "down", "Error: connection refused")
	out := RenderTemplate("{{name}} {{target}} {{error}} {{status_code}}", ctx)
	if !strings.Contains(out, "API") || !strings.Contains(out, "https://x.com") {
		t.Fatalf("unexpected: %q", out)
	}
	if !strings.Contains(out, "connection refused") || !strings.Contains(out, "503") {
		t.Fatalf("missing fields: %q", out)
	}
}

func TestRenderTemplate_unknownPlaceholderLeft(t *testing.T) {
	ctx := TestTemplateContext()
	out := RenderTemplate("{{unknown_key}}", ctx)
	if out != "{{unknown_key}}" {
		t.Fatalf("got %q", out)
	}
}

func TestPreviewTemplateContext_downAndRecovered(t *testing.T) {
	down := PreviewTemplateContext("down")
	if down.Status != "down" || down.Error == "" {
		t.Fatalf("down ctx: %+v", down)
	}
	rec := PreviewTemplateContext("recovered")
	if rec.Status != "recovered" || rec.Latency == 0 {
		t.Fatalf("recovered ctx: %+v", rec)
	}
}

func TestRenderTemplate_previewDown(t *testing.T) {
	ctx := PreviewTemplateContext("down")
	out := RenderTemplate("{{name}} {{status}} {{error}}", ctx)
	if !strings.Contains(out, "Beacon (preview)") || !strings.Contains(out, "down") {
		t.Fatalf("unexpected: %q", out)
	}
}
