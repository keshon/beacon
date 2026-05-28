package notify

import (
	"strings"

	"github.com/keshon/beacon/internal/config"
)

const (
	AlertModeRepeat = "repeat"
	AlertModeOnce   = "once"
)

// MaxTemplateLen caps custom template size.
const MaxTemplateLen = 2000

// ResolvedPolicy is the effective notification policy for one receiver.
type ResolvedPolicy struct {
	AlertMode string
	Templates config.MessageTemplates
}

// DefaultTemplates returns built-in down/recovered templates.
func DefaultTemplates() config.MessageTemplates {
	return config.DefaultMessageTemplates()
}

// DefaultAlertMode is used when nothing is configured.
func DefaultAlertMode() string {
	return AlertModeRepeat
}

// TemplateForStatus picks the template string for an alert status.
func (p ResolvedPolicy) TemplateForStatus(status string) string {
	switch status {
	case "recovered":
		return p.Templates.Recovered
	default:
		return p.Templates.Down
	}
}

// ResolveReceiverPolicy merges row policy with global notifications defaults.
// Row non-empty fields win; then global; then built-in defaults.
func ResolveReceiverPolicy(cfg *config.Config, row *config.ReceiverPolicy) ResolvedPolicy {
	def := DefaultTemplates()
	globalMode := ""
	globalTpl := config.MessageTemplates{}
	if cfg != nil {
		globalMode = cfg.Notifications.AlertMode
		globalTpl = cfg.Notifications.Templates
	}
	rowMode := ""
	rowTpl := config.MessageTemplates{}
	if row != nil {
		rowMode = row.AlertMode
		if row.Templates != nil {
			rowTpl = SanitizeTemplates(row.Templates)
		}
	}
	return ResolvedPolicy{
		AlertMode: mergeAlertMode(rowMode, globalMode, DefaultAlertMode()),
		Templates: mergeTemplates(def, globalTpl, rowTpl),
	}
}

// IsCustomTemplates reports whether effective templates differ from built-in defaults.
func IsCustomTemplates(p ResolvedPolicy) bool {
	def := DefaultTemplates()
	return p.Templates.Down != def.Down || p.Templates.Recovered != def.Recovered
}

func mergeAlertMode(rowMode, globalMode, fallback string) string {
	if s := strings.TrimSpace(rowMode); s != "" {
		if normalized := normalizeAlertMode(s); normalized != "" {
			return normalized
		}
	}
	if s := strings.TrimSpace(globalMode); s != "" {
		if normalized := normalizeAlertMode(s); normalized != "" {
			return normalized
		}
	}
	return fallback
}

func normalizeAlertMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case AlertModeRepeat, AlertModeOnce:
		return strings.ToLower(strings.TrimSpace(mode))
	default:
		return ""
	}
}

// mergeTemplates: for each field, prefer b (higher priority) if non-empty, else a, else def.
func mergeTemplates(def, a, b config.MessageTemplates) config.MessageTemplates {
	return config.MessageTemplates{
		Down:      pickTemplate(def.Down, a.Down, b.Down),
		Recovered: pickTemplate(def.Recovered, a.Recovered, b.Recovered),
	}
}

func pickTemplate(def, a, b string) string {
	if s := strings.TrimSpace(b); s != "" {
		return capTemplate(s)
	}
	if s := strings.TrimSpace(a); s != "" {
		return capTemplate(s)
	}
	return def
}

func capTemplate(s string) string {
	if len(s) > MaxTemplateLen {
		return s[:MaxTemplateLen]
	}
	return s
}

// SanitizeTemplates trims and caps template fields.
func SanitizeTemplates(t *config.MessageTemplates) config.MessageTemplates {
	if t == nil {
		return config.MessageTemplates{}
	}
	return config.MessageTemplates{
		Down:      capTemplate(strings.TrimSpace(t.Down)),
		Recovered: capTemplate(strings.TrimSpace(t.Recovered)),
	}
}

// PlaceholderInfo describes a template variable for the UI.
type PlaceholderInfo struct {
	Key         string `json:"key"`
	Description string `json:"description"`
}

// Placeholders returns the supported {{key}} list.
func Placeholders() []PlaceholderInfo {
	return []PlaceholderInfo{
		{Key: "name", Description: "Monitor name"},
		{Key: "target", Description: "URL or host:port"},
		{Key: "type", Description: "Check type (http, tcp)"},
		{Key: "status", Description: "Alert status (down, recovered, test)"},
		{Key: "error", Description: "Check error text"},
		{Key: "latency", Description: "Response latency"},
		{Key: "status_code", Description: "HTTP status code (0 if N/A)"},
		{Key: "time", Description: "Event time"},
		{Key: "message", Description: "Detail line (error or latency summary)"},
		{Key: "fail_count", Description: "Failed check count before down"},
	}
}
