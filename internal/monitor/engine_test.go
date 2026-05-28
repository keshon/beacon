package monitor

import (
	"testing"
	"time"

	"github.com/keshon/beacon/internal/checks"
)

func TestEngine_downCallback_firstAndRepeat(t *testing.T) {
	var firstCalls, repeatCalls, recoverCalls int
	e := NewEngine(
		func(m *Monitor, state *MonitorState, result checks.CheckResult, isRepeat bool) {
			if isRepeat {
				repeatCalls++
			} else {
				firstCalls++
			}
		},
		func(m *Monitor, state *MonitorState, result checks.CheckResult) { recoverCalls++ },
	)
	m := &Monitor{ID: "1", Name: "t", Retries: 1}
	st := &MonitorState{MonitorID: "1", Status: StatusUnknown}
	fail := checks.CheckResult{Success: false, Error: "timeout", Time: time.Now()}

	e.Process(fail, st, m)
	if firstCalls != 1 {
		t.Fatalf("first down: want 1, got %d", firstCalls)
	}
	e.Process(fail, st, m)
	e.Process(fail, st, m)
	if repeatCalls != 2 {
		t.Fatalf("repeat while down: want 2, got %d", repeatCalls)
	}
	ok := checks.CheckResult{Success: true, Latency: time.Millisecond, Time: time.Now()}
	e.Process(ok, st, m)
	if recoverCalls != 1 {
		t.Fatalf("recovery: want 1, got %d", recoverCalls)
	}
}
