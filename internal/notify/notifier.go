package notify

import (
	"strings"
	"time"
)

type Alert struct {
	MonitorName string
	Status      string
	Message     string
	Body        string // rendered template; empty falls back to default templates
	Time        time.Time
	Target      string
	Type        string
	StatusCode  int
	Latency     time.Duration
	FailCount   int
}

type Notifier interface {
	Send(Alert) error
}

// AlertText returns the message body to send.
func AlertText(a Alert) string {
	if s := strings.TrimSpace(a.Body); s != "" {
		return s
	}
	status := a.Status
	if status != "recovered" {
		status = "down"
	}
	return BuildAlertBody(ResolvedPolicy{Templates: DefaultTemplates()}, status, TemplateContextFromAlert(a))
}
