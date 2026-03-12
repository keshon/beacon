package monitor

import (
	"github.com/keshon/beacon/internal/checks"
)

type StateHandler func(result checks.CheckResult, state *MonitorState, m *Monitor)

type Engine struct {
	onDown    func(m *Monitor, state *MonitorState, result checks.CheckResult)
	onRecover func(m *Monitor, state *MonitorState, result checks.CheckResult)
}

func NewEngine(onDown, onRecover func(*Monitor, *MonitorState, checks.CheckResult)) *Engine {
	return &Engine{
		onDown:    onDown,
		onRecover: onRecover,
	}
}

func (e *Engine) Process(result checks.CheckResult, state *MonitorState, m *Monitor) {
	if state == nil {
		state = &MonitorState{
			MonitorID: m.ID,
			Status:    StatusUnknown,
		}
	}

	state.LastCheck = result.Time

	if result.Success {
		// OK result
		if state.Status == StatusDown {
			// DOWN -> UP: recovery
			state.Status = StatusUp
			state.FailCount = 0
			state.LastSuccess = result.Time
			state.Latency = result.Latency
			if e.onRecover != nil {
				e.onRecover(m, state, result)
			}
		} else {
			// UP -> OK: update latency
			state.Status = StatusUp
			state.FailCount = 0
			state.LastSuccess = result.Time
			state.Latency = result.Latency
		}
	} else {
		// FAIL result
		if state.Status == StatusDown {
			// DOWN -> FAIL: notify on every failed poll when already down
			if e.onDown != nil {
				e.onDown(m, state, result)
			}
			return
		}
		state.FailCount++
		if state.FailCount >= m.Retries {
			state.Status = StatusDown
			if e.onDown != nil {
				e.onDown(m, state, result)
			}
		}
	}
}
