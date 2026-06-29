package cluster

import (
	"crypto/subtle"
	"log"
	"net/http"
	"strings"

	"github.com/keshon/beacon/internal/config"
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

func setOutboundSyncAuth(req *http.Request, cfg *config.Config) {
	if cfg == nil || req == nil {
		return
	}
	if token := strings.TrimSpace(cfg.Network.SyncToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		return
	}
	pw := cfg.Auth.PasswordForBasicAuth()
	if pw == "" {
		log.Printf("[cluster] warning: no sync_token and no web password for outbound peer auth")
	}
	req.SetBasicAuth(cfg.Auth.Username, pw)
}
