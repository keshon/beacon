package config

import "strings"

// MaskSecret returns a display-safe preview of a secret (first/last runes visible).
func MaskSecret(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	runes := []rune(s)
	n := len(runes)
	if n <= 8 {
		return strings.Repeat("•", n)
	}
	var b strings.Builder
	b.Grow(n)
	for i, r := range runes {
		if i < 4 || i >= n-4 {
			b.WriteRune(r)
		} else {
			b.WriteRune('•')
		}
	}
	return b.String()
}

// SecretUnchanged reports whether incoming is empty or matches the stored secret
// (including a masked preview from GET /api/config).
func SecretUnchanged(incoming, stored string) bool {
	stored = strings.TrimSpace(stored)
	if stored == "" {
		return false
	}
	incoming = strings.TrimSpace(incoming)
	if incoming == "" {
		return true
	}
	if incoming == stored {
		return true
	}
	return incoming == MaskSecret(stored)
}
