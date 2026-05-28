package notify

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/keshon/beacon/internal/checks"
	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
)

// TemplateContext supplies values for {{placeholder}} substitution.
type TemplateContext struct {
	MonitorName string
	Target      string
	Type        string
	Status      string
	Error       string
	Latency     time.Duration
	StatusCode  int
	Time        time.Time
	Message     string
	FailCount   int
}

// NewTemplateContext builds context from a check result and monitor state.
func NewTemplateContext(m *monitor.Monitor, state *monitor.MonitorState, result checks.CheckResult, status, message string) TemplateContext {
	ctx := TemplateContext{
		MonitorName: "",
		Target:      "",
		Type:        "",
		Status:      status,
		Error:       result.Error,
		Latency:     result.Latency,
		StatusCode:  result.StatusCode,
		Time:        result.Time,
		Message:     message,
		FailCount:   0,
	}
	if m != nil {
		ctx.MonitorName = m.Name
		ctx.Target = m.Target
		ctx.Type = m.Type
	}
	if state != nil {
		ctx.FailCount = state.FailCount
	}
	if ctx.Time.IsZero() {
		ctx.Time = time.Now()
	}
	return ctx
}

// PreviewTemplateContext returns sample placeholder values for template preview sends.
func PreviewTemplateContext(status string) TemplateContext {
	ctx := TemplateContext{
		MonitorName: "Beacon (preview)",
		Target:      "https://example.com",
		Type:        "http",
		Status:      status,
		Time:        time.Now(),
		StatusCode:  503,
		FailCount:   2,
	}
	switch status {
	case "recovered":
		ctx.Latency = 42 * time.Millisecond
		ctx.Message = "Latency: 42ms"
		ctx.StatusCode = 200
		ctx.FailCount = 0
	default:
		ctx.Status = "down"
		ctx.Error = "connection timed out"
		ctx.Message = "Error: connection timed out"
	}
	return ctx
}

// TestTemplateContext is kept for unit tests.
func TestTemplateContext() TemplateContext {
	return PreviewTemplateContext("down")
}

func (c TemplateContext) values() map[string]string {
	return map[string]string{
		"name":         c.MonitorName,
		"target":       c.Target,
		"type":         c.Type,
		"status":       c.Status,
		"error":        c.Error,
		"latency":      c.Latency.String(),
		"status_code":  strconv.Itoa(c.StatusCode),
		"time":         c.Time.Format("2006-01-02 15:04"),
		"message":      c.Message,
		"fail_count":   strconv.Itoa(c.FailCount),
	}
}

// RenderTemplate replaces {{key}} placeholders; unknown keys are left unchanged.
func RenderTemplate(tpl string, ctx TemplateContext) string {
	if tpl == "" {
		return ""
	}
	vals := ctx.values()
	var b strings.Builder
	i := 0
	for i < len(tpl) {
		start := strings.Index(tpl[i:], "{{")
		if start < 0 {
			b.WriteString(tpl[i:])
			break
		}
		start += i
		b.WriteString(tpl[i:start])
		end := strings.Index(tpl[start:], "}}")
		if end < 0 {
			b.WriteString(tpl[start:])
			break
		}
		end += start
		key := strings.TrimSpace(tpl[start+2 : end])
		if v, ok := vals[key]; ok {
			b.WriteString(v)
		} else {
			b.WriteString(tpl[start : end+2])
		}
		i = end + 2
	}
	return b.String()
}

// BuildAlertBody renders the template for status using resolved policy.
func BuildAlertBody(policy ResolvedPolicy, status string, ctx TemplateContext) string {
	tpl := policy.TemplateForStatus(status)
	if tpl == "" {
		return ""
	}
	return RenderTemplate(tpl, ctx)
}

// NotifyOverrideHasPolicy returns true if any override receiver row has policy.
func NotifyOverrideHasPolicy(o *monitor.NotifyOverride) bool {
	if o == nil {
		return false
	}
	for _, t := range o.Telegram {
		if receiverPolicyConfigured(t.Policy) {
			return true
		}
	}
	for _, d := range o.Discord {
		if receiverPolicyConfigured(d.Policy) {
			return true
		}
	}
	return false
}

func receiverPolicyConfigured(p *config.ReceiverPolicy) bool {
	if p == nil {
		return false
	}
	if strings.TrimSpace(p.AlertMode) != "" {
		return true
	}
	if p.Templates != nil {
		t := SanitizeTemplates(p.Templates)
		return t.Down != "" || t.Recovered != ""
	}
	return false
}

// FormatLegacyAlert keeps backward-compatible formatting when Body is empty.
func FormatLegacyAlert(a Alert) string {
	switch a.Status {
	case "recovered":
		return fmt.Sprintf("Site RECOVERED\n\n%s\n%s\nTime: %s",
			a.MonitorName, a.Message, a.Time.Format("2006-01-02 15:04"))
	case "test":
		return fmt.Sprintf("Beacon TEST\n\n%s\n%s\nTime: %s",
			a.MonitorName, a.Message, a.Time.Format("2006-01-02 15:04"))
	}
	return fmt.Sprintf("Site DOWN\n\n%s\n%s\nTime: %s",
		a.MonitorName, a.Message, a.Time.Format("2006-01-02 15:04"))
}
