package notify

import "time"

// staggerDelay spaces real fan-out sends so a single monitor flip with several
// recipients does not hit provider burst rate limits at once.
const staggerDelay = 250 * time.Millisecond

// SendAll delivers alert through every notifier sequentially, pausing briefly
// between recipients. It returns the per-notifier errors in input order with
// nil entries on success.
func SendAll(notifiers []Notifier, a Alert) []error {
	if len(notifiers) == 0 {
		return nil
	}
	errs := make([]error, len(notifiers))
	for i, n := range notifiers {
		if i > 0 {
			time.Sleep(staggerDelay)
		}
		errs[i] = n.Send(a)
	}
	return errs
}

// TestAlert returns a mock alert used by the "Test" buttons in the UI.
func TestAlert() Alert {
	return Alert{
		MonitorName: "Beacon (test)",
		Status:      "test",
		Message:     "This is a test notification. If you see this, delivery works.",
		Time:        time.Now(),
	}
}
