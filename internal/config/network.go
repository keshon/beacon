package config

import (
	"net/url"
	"strings"
)

// NodeDomain returns the hostname from SelfURL (e.g. beacon.example.com).
func (n NetworkConfig) NodeDomain() string {
	raw := strings.TrimSpace(n.SelfURL)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(u.Hostname())
}
