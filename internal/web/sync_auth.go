package web

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

const syncTokenHeader = "X-Beacon-Sync-Token"

// SyncTokenFromRequest reads a peer sync token from Bearer or X-Beacon-Sync-Token.
func SyncTokenFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	if h := strings.TrimSpace(r.Header.Get(syncTokenHeader)); h != "" {
		return h
	}
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}
	return ""
}

// SyncTokenMatches reports whether the request carries the expected sync token.
func SyncTokenMatches(r *http.Request, expected string) bool {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return false
	}
	got := SyncTokenFromRequest(r)
	if got == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}
