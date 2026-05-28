package notify

// ShouldSendDown returns whether a down alert should be delivered for the
// given engine event. First transition (isRepeat false) always sends; repeat
// polls only send when the receiver policy is repeat mode.
func ShouldSendDown(policy ResolvedPolicy, isRepeat bool) bool {
	if !isRepeat {
		return true
	}
	return policy.AlertMode == AlertModeRepeat
}
