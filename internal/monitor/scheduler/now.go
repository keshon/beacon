package scheduler

import (
	"errors"
	"time"
)

// Checking on demand.
//
// Until now the interface could only tell a person things. It said a monitor
// was down and offered nowhere to answer "check it again, I just fixed it" —
// so the answer was to wait out the interval, or to reload the page hoping the
// schedule had come round. A conversation where one side only talks is not a
// conversation.
//
// The check runs through the SAME queue as a scheduled one: same worker, same
// in-flight guard, same recording. A second path would drift from the first,
// and the one people reach for in a hurry is the worst place for a divergence.

var (
	// ErrCheckInFlight means the monitor is already being checked. Not an
	// error the person needs to fix — the thing they asked for is happening.
	ErrCheckInFlight = errors.New("check already running")
	// ErrQueueFull means every worker is busy. Honest to report: the answer is
	// "not now", not a silent no-op.
	ErrQueueFull = errors.New("check queue is full")
)

// checkNowWait is how long an on-demand check waits for a free worker. Short:
// a person is watching, and a spinner that outlives their patience is worse
// than being told to try again.
const checkNowWait = 2 * time.Second

// CheckNow queues an immediate check for one monitor.
func (sc *Scheduler) CheckNow(monitorID string) error {
	if sc == nil || sc.stopping.Load() {
		return ErrQueueFull
	}

	sc.inflMu.Lock()
	if _, running := sc.inFlight[monitorID]; running {
		sc.inflMu.Unlock()
		return ErrCheckInFlight
	}
	sc.inFlight[monitorID] = struct{}{}
	sc.inflMu.Unlock()

	job := CheckJob{
		MonitorID:       monitorID,
		RequireLocalMon: sc.source.RequireLocalMonitor(monitorID),
	}
	select {
	case sc.jobs <- job:
		return nil
	case <-time.After(checkNowWait):
		sc.inflMu.Lock()
		delete(sc.inFlight, monitorID)
		sc.inflMu.Unlock()
		return ErrQueueFull
	}
}
