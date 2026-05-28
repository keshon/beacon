package notify

import (
	"strings"
	"time"
)

type Alert struct {
	MonitorName string
	Status      string
	Message     string
	Body        string // rendered template; empty uses legacy formatAlert
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
	if strings.TrimSpace(a.Body) != "" {
		return a.Body
	}
	return FormatLegacyAlert(a)
}
