package monitor

import (
	"github.com/keshon/beacon/internal/checks"
)

type Engine struct {
	onDown    func(m *Monitor, state *MonitorState, result checks.CheckResult, isRepeat bool)
	onRecover func(m *Monitor, state *MonitorState, result checks.CheckResult)
}

func NewEngine(
	onDown func(*Monitor, *MonitorState, checks.CheckResult, bool),
	onRecover func(*Monitor, *MonitorState, checks.CheckResult),
) *Engine {
	return &Engine{
		onDown:    onDown,
		onRecover: onRecover,
	}
}

// Process updates state from a check result. onDown receives isRepeat=true
// when the monitor is already down and another failed check occurred.
func (e *Engine) Process(result checks.CheckResult, state *MonitorState, m *Monitor) {
	if state == nil {
		state = &MonitorState{
			MonitorID: m.ID,
			Status:    StatusUnknown,
		}
	}

	state.LastCheck = result.Time

	if result.Success {
		if state.Status == StatusDown {
			state.Status = StatusUp
			state.FailCount = 0
			state.LastSuccess = result.Time
			state.Latency = result.Latency
			if e.onRecover != nil {
				e.onRecover(m, state, result)
			}
		} else {
			state.Status = StatusUp
			state.FailCount = 0
			state.LastSuccess = result.Time
			state.Latency = result.Latency
		}
	} else {
		if state.Status == StatusDown {
			if e.onDown != nil {
				e.onDown(m, state, result, true)
			}
			return
		}
		state.FailCount++
		if state.FailCount >= m.Retries {
			state.Status = StatusDown
			if e.onDown != nil {
				e.onDown(m, state, result, false)
			}
		}
	}
}
