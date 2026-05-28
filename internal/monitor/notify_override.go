package monitor

import (
	"strings"

	"github.com/keshon/beacon/internal/config"
)

// MigrateNotifyOverride copies deprecated top-level alert_mode/templates onto
// each receiver row that has no policy, then clears the legacy fields.
func MigrateNotifyOverride(n *NotifyOverride) {
	if n == nil {
		return
	}
	legacyMode := strings.TrimSpace(n.AlertMode)
	var legacyTpl *config.MessageTemplates
	if n.Templates != nil {
		t := *n.Templates
		if strings.TrimSpace(t.Down) != "" || strings.TrimSpace(t.Recovered) != "" {
			legacyTpl = &t
		}
	}
	if legacyMode == "" && legacyTpl == nil {
		return
	}
	legacy := &config.ReceiverPolicy{
		AlertMode: legacyMode,
		Templates: legacyTpl,
	}
	for i := range n.Telegram {
		if receiverPolicyEmpty(n.Telegram[i].Policy) {
			n.Telegram[i].Policy = cloneReceiverPolicy(legacy)
		}
	}
	for i := range n.Discord {
		if receiverPolicyEmpty(n.Discord[i].Policy) {
			n.Discord[i].Policy = cloneReceiverPolicy(legacy)
		}
	}
	n.AlertMode = ""
	n.Templates = nil
}

func receiverPolicyEmpty(p *config.ReceiverPolicy) bool {
	if p == nil {
		return true
	}
	if strings.TrimSpace(p.AlertMode) != "" {
		return false
	}
	if p.Templates == nil {
		return true
	}
	return strings.TrimSpace(p.Templates.Down) == "" && strings.TrimSpace(p.Templates.Recovered) == ""
}

func cloneReceiverPolicy(p *config.ReceiverPolicy) *config.ReceiverPolicy {
	if p == nil {
		return nil
	}
	out := &config.ReceiverPolicy{AlertMode: strings.TrimSpace(p.AlertMode)}
	if p.Templates != nil {
		t := *p.Templates
		out.Templates = &t
	}
	if out.AlertMode == "" && out.Templates == nil {
		return nil
	}
	return out
}
