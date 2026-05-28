package notify

import "testing"

func TestShouldSendDown(t *testing.T) {
	repeat := ResolvedPolicy{AlertMode: AlertModeRepeat}
	once := ResolvedPolicy{AlertMode: AlertModeOnce}

	if !ShouldSendDown(repeat, false) || !ShouldSendDown(once, false) {
		t.Fatal("first down should always send")
	}
	if !ShouldSendDown(repeat, true) {
		t.Fatal("repeat receiver should send on repeat down")
	}
	if ShouldSendDown(once, true) {
		t.Fatal("once receiver should skip repeat down")
	}
}
