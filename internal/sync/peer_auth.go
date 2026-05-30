package sync

import (
	"log"
	"net/http"
	"strings"

	"github.com/keshon/beacon/internal/config"
)

func setPeerSyncAuth(req *http.Request, cfg *config.Config) {
	if cfg == nil || req == nil {
		return
	}
	if token := strings.TrimSpace(cfg.Network.SyncToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		return
	}
	pw := cfg.Auth.PasswordForBasicAuth()
	if pw == "" {
		log.Printf("[sync] warning: no sync_token and no web password for outbound peer auth")
	}
	req.SetBasicAuth(cfg.Auth.Username, pw)
}
