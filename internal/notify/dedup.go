package notify

import (
	"sync"
	"time"
)

// AlertDedup suppresses duplicate alerts for the same monitor status within a window.
type AlertDedup struct {
	mu   sync.Mutex
	last map[string]time.Time
}

func NewAlertDedup() *AlertDedup {
	return &AlertDedup{last: make(map[string]time.Time)}
}

func alertDedupKey(monitorID, status string) string {
	return monitorID + "\x00" + status
}

// Allow reports whether an alert should be sent (false if duplicate within window).
func (d *AlertDedup) Allow(monitorID, status string, window time.Duration) bool {
	key := alertDedupKey(monitorID, status)
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	if prev, ok := d.last[key]; ok && now.Sub(prev) < window {
		return false
	}
	d.last[key] = now
	return true
}
