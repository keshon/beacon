package notify

import "time"

type Alert struct {
	MonitorName string
	Status      string
	Message     string
	Time        time.Time
}

type Notifier interface {
	Send(Alert) error
}
