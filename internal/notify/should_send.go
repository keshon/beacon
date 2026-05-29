package notify

// ShouldSendDown returns whether a down alert should be delivered.
// Email always uses once semantics regardless of policy alert_mode.
func ShouldSendDown(policy ResolvedPolicy, isRepeat bool, channel string) bool {
	if channel == ChannelEmail {
		return !isRepeat
	}
	if !isRepeat {
		return true
	}
	return policy.AlertMode == AlertModeRepeat
}
