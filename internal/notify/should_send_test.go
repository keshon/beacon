package notify

import "testing"

func TestShouldSendDown(t *testing.T) {
	repeat := ResolvedPolicy{AlertMode: AlertModeRepeat}
	once := ResolvedPolicy{AlertMode: AlertModeOnce}

	if !ShouldSendDown(repeat, false, ChannelTelegram) || !ShouldSendDown(once, false, ChannelDiscord) {
		t.Fatal("first down should always send")
	}
	if !ShouldSendDown(repeat, true, ChannelTelegram) {
		t.Fatal("repeat receiver should send on repeat down")
	}
	if ShouldSendDown(once, true, ChannelDiscord) {
		t.Fatal("once receiver should skip repeat down")
	}
	if ShouldSendDown(repeat, true, ChannelEmail) {
		t.Fatal("email should never repeat down")
	}
	if !ShouldSendDown(once, false, ChannelEmail) {
		t.Fatal("email should send first down")
	}
}
