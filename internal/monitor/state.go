package monitor

import "time"

type MonitorState struct {
	MonitorID   string        `json:"monitor_id"`
	Status      string        `json:"status"` // up, down, unknown
	FailCount   int           `json:"fail_count"`
	LastCheck   time.Time     `json:"last_check"`
	LastSuccess time.Time     `json:"last_success"`
	Latency     time.Duration `json:"latency"`
}

const (
	StatusUp      = "up"
	StatusDown    = "down"
	StatusUnknown = "unknown"
)
