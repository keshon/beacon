package checks

import "time"

type CheckResult struct {
	MonitorID   string
	Success     bool
	StatusCode  int
	Latency     time.Duration
	Error       string
	Time        time.Time
}
