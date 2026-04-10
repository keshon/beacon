package monitor

import "time"

// NotifyOverride holds per-monitor notification overrides.
type NotifyOverride struct {
	Telegram *struct {
		Token  string `json:"token,omitempty"`
		ChatID string `json:"chat_id,omitempty"`
	} `json:"telegram,omitempty"`
	Discord *struct {
		Webhook string `json:"webhook,omitempty"`
	} `json:"discord,omitempty"`
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
	Notify         []string        `json:"notify"` // telegram, discord
	NotifyOverride *NotifyOverride `json:"notify_override,omitempty"`
	OwnerNodeID    string          `json:"owner_node_id,omitempty"` // empty = local/legacy
}
