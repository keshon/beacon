package notify

import (
	"time"

	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
)

const (
	minIntervalTelegram = 20 * time.Second
	minIntervalDiscord  = 60 * time.Second
	minIntervalWebhook  = 30 * time.Second
	minIntervalProbe    = 5 * time.Second
)

// RecommendedMinInterval returns a conservative minimum check interval for a monitor.
// Email is excluded (once-only delivery with separate send guard).
func RecommendedMinInterval(cfg *config.Config, m *monitor.Monitor) time.Duration {
	recvs := BuildReceivers(cfg, m)
	if len(recvs) == 0 {
		return minIntervalProbe
	}
	var max time.Duration
	hasRepeat := false
	for _, r := range recvs {
		if r.Channel == ChannelEmail {
			continue
		}
		floor := channelIntervalFloor(r.Channel)
		if floor > max {
			max = floor
		}
		if r.Policy.AlertMode == AlertModeRepeat {
			hasRepeat = true
		}
	}
	if !hasRepeat {
		return minIntervalProbe
	}
	if max == 0 {
		return minIntervalProbe
	}
	return max
}

func channelIntervalFloor(channel string) time.Duration {
	switch channel {
	case ChannelTelegram:
		return minIntervalTelegram
	case ChannelDiscord:
		return minIntervalDiscord
	case ChannelWebhook:
		return minIntervalWebhook
	default:
		return 0
	}
}

// IntervalWarnings returns human-readable warnings when monitor interval is below recommended.
func IntervalWarnings(cfg *config.Config, m *monitor.Monitor) []string {
	if m == nil {
		return nil
	}
	rec := RecommendedMinInterval(cfg, m)
	if m.Interval <= 0 {
		return nil
	}
	if m.Interval >= rec {
		return nil
	}
	return []string{
		"check interval " + m.Interval.String() + " is below recommended minimum " + rec.String() + " for repeat-mode notification channels",
	}
}
