package monitor

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/keshon/beacon/internal/monitor/checks"
	"github.com/keshon/beacon/internal/config"
)

// NotifyOverride holds per-monitor notification routing per channel.
type NotifyOverride struct {
	Telegram  *TelegramChannelOverride  `json:"telegram,omitempty"`
	Discord   *DiscordChannelOverride   `json:"discord,omitempty"`
	Email     *EmailChannelOverride     `json:"email,omitempty"`
	Webhook   *WebhookChannelOverride   `json:"webhook,omitempty"`
	AlertMode string                    `json:"alert_mode,omitempty"` // deprecated
	Templates *config.MessageTemplates  `json:"templates,omitempty"`  // deprecated
}

// UnmarshalJSON accepts tri-state channel blocks and legacy slice shapes.
func (n *NotifyOverride) UnmarshalJSON(data []byte) error {
	*n = NotifyOverride{}
	var raw struct {
		Telegram  json.RawMessage          `json:"telegram"`
		Discord   json.RawMessage          `json:"discord"`
		Email     json.RawMessage          `json:"email"`
		Webhook   json.RawMessage          `json:"webhook"`
		AlertMode string                   `json:"alert_mode"`
		Templates *config.MessageTemplates `json:"templates"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	n.AlertMode = raw.AlertMode
	n.Templates = raw.Templates
	n.Telegram = unmarshalTelegramChannel(raw.Telegram)
	n.Discord = unmarshalDiscordChannel(raw.Discord)
	n.Email = unmarshalEmailChannel(raw.Email)
	n.Webhook = unmarshalWebhookChannel(raw.Webhook)
	return nil
}

func unmarshalTelegramChannel(data json.RawMessage) *TelegramChannelOverride {
	if len(data) == 0 {
		return nil
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var targets []config.TelegramTarget
		if err := json.Unmarshal(data, &targets); err != nil {
			return nil
		}
		if len(targets) == 0 {
			return &TelegramChannelOverride{Mode: NotifyChannelInherit}
		}
		return &TelegramChannelOverride{Mode: NotifyChannelCustom, Targets: targets}
	}
	var ch TelegramChannelOverride
	if err := json.Unmarshal(data, &ch); err != nil {
		return nil
	}
	if ch.Mode == "" && len(ch.Targets) > 0 {
		ch.Mode = NotifyChannelCustom
	}
	if ch.Mode == "" {
		ch.Mode = NotifyChannelInherit
	}
	return &ch
}

func unmarshalDiscordChannel(data json.RawMessage) *DiscordChannelOverride {
	if len(data) == 0 {
		return nil
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	if strings.HasPrefix(trimmed, "[") {
		targets, err := config.ParseDiscordReceiversJSON(data)
		if err != nil || len(targets) == 0 {
			if len(targets) == 0 {
				return &DiscordChannelOverride{Mode: NotifyChannelInherit}
			}
			return nil
		}
		return &DiscordChannelOverride{Mode: NotifyChannelCustom, Targets: targets}
	}
	var ch DiscordChannelOverride
	if err := json.Unmarshal(data, &ch); err != nil {
		return nil
	}
	if ch.Mode == "" && len(ch.Targets) > 0 {
		ch.Mode = NotifyChannelCustom
	}
	if ch.Mode == "" {
		ch.Mode = NotifyChannelInherit
	}
	return &ch
}

func unmarshalEmailChannel(data json.RawMessage) *EmailChannelOverride {
	if len(data) == 0 {
		return nil
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var targets []config.EmailTarget
		if err := json.Unmarshal(data, &targets); err != nil {
			return nil
		}
		if len(targets) == 0 {
			return &EmailChannelOverride{Mode: NotifyChannelInherit}
		}
		return &EmailChannelOverride{Mode: NotifyChannelCustom, Targets: targets}
	}
	var ch EmailChannelOverride
	if err := json.Unmarshal(data, &ch); err != nil {
		return nil
	}
	if ch.Mode == "" && len(ch.Targets) > 0 {
		ch.Mode = NotifyChannelCustom
	}
	if ch.Mode == "" {
		ch.Mode = NotifyChannelInherit
	}
	return &ch
}

func unmarshalWebhookChannel(data json.RawMessage) *WebhookChannelOverride {
	if len(data) == 0 {
		return nil
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var targets []config.WebhookReceiver
		if err := json.Unmarshal(data, &targets); err != nil {
			return nil
		}
		if len(targets) == 0 {
			return &WebhookChannelOverride{Mode: NotifyChannelInherit}
		}
		return &WebhookChannelOverride{Mode: NotifyChannelCustom, Targets: targets}
	}
	var ch WebhookChannelOverride
	if err := json.Unmarshal(data, &ch); err != nil {
		return nil
	}
	if ch.Mode == "" && len(ch.Targets) > 0 {
		ch.Mode = NotifyChannelCustom
	}
	if ch.Mode == "" {
		ch.Mode = NotifyChannelInherit
	}
	return &ch
}

type Monitor struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Type           string          `json:"type"` // http, tcp
	Target         string          `json:"target"`
	Interval       time.Duration   `json:"interval"`
	Timeout        time.Duration   `json:"timeout"`
	Retries        int             `json:"retries"`
	Enabled        bool            `json:"enabled"`
	Notify         []string        `json:"notify"` // deprecated
	HTTP           *checks.HTTPOptions `json:"http,omitempty"`
	NotifyOverride *NotifyOverride `json:"notify_override,omitempty"`
	OwnerNodeID    string          `json:"owner_node_id,omitempty"`
}

// Redacted returns a copy safe for API responses (HTTP password omitted).
func (m *Monitor) Redacted() *Monitor {
	if m == nil {
		return nil
	}
	out := *m
	if out.HTTP != nil {
		out.HTTP = out.HTTP.Redacted()
	}
	return &out
}

// HasLegacyNotifyFields reports deprecated top-level notify_override fields.
func HasLegacyNotifyFields(n *NotifyOverride) bool {
	if n == nil {
		return false
	}
	if strings.TrimSpace(n.AlertMode) != "" {
		return true
	}
	if n.Templates != nil {
		if strings.TrimSpace(n.Templates.Down) != "" || strings.TrimSpace(n.Templates.Recovered) != "" {
			return true
		}
	}
	return false
}

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
	if n.Telegram != nil {
		for i := range n.Telegram.Targets {
			if receiverPolicyEmpty(n.Telegram.Targets[i].Policy) {
				n.Telegram.Targets[i].Policy = cloneReceiverPolicy(legacy)
			}
		}
	}
	if n.Discord != nil {
		for i := range n.Discord.Targets {
			if receiverPolicyEmpty(n.Discord.Targets[i].Policy) {
				n.Discord.Targets[i].Policy = cloneReceiverPolicy(legacy)
			}
		}
	}
	if n.Email != nil {
		for i := range n.Email.Targets {
			if receiverPolicyEmpty(n.Email.Targets[i].Policy) {
				n.Email.Targets[i].Policy = cloneReceiverPolicy(legacy)
			}
		}
	}
	if n.Webhook != nil {
		for i := range n.Webhook.Targets {
			if receiverPolicyEmpty(n.Webhook.Targets[i].Policy) {
				n.Webhook.Targets[i].Policy = cloneReceiverPolicy(legacy)
			}
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

type MonitorState struct {
	MonitorID   string        `json:"monitor_id"`
	Status      string        `json:"status"` // up, down, unknown
	FailCount   int           `json:"fail_count"`
	LastCheck   time.Time     `json:"last_check"`
	LastSuccess time.Time     `json:"last_success"`
	Latency     time.Duration `json:"latency"`
	// CertExpiry is the deadline of the certificate seen on the last successful
	// check. It lives on the state rather than in the history because only the
	// current one matters: nobody needs to know what the certificate was.
	//
	// No omitempty: it does nothing on a time.Time — the zero value is a struct,
	// not an empty one — and a tag that reads as a promise it cannot keep is
	// worse than no tag.
	CertExpiry time.Time `json:"cert_expiry"`
	// Phases is where the last check spent its time. On the state and not in
	// the history for the same reason as the certificate: the question is
	// "why is it slow now", and a month of breakdowns answers a different one.
	Phases CheckPhases `json:"phases"`
}

// CheckPhases mirrors checks.Phases without importing it: package monitor is
// what checks depends on, and the arrow must not point back.
type CheckPhases struct {
	DNS    time.Duration `json:"dns"`
	TCP    time.Duration `json:"tcp"`
	TLS    time.Duration `json:"tls"`
	Server time.Duration `json:"server"`
}

// Any reports whether anything was measured.
func (p CheckPhases) Any() bool {
	return p.DNS > 0 || p.TCP > 0 || p.TLS > 0 || p.Server > 0
}

const (
	StatusUp      = "up"
	StatusDown    = "down"
	StatusUnknown = "unknown"
)
