package sync

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/keshon/beacon/internal/config"
)

func TestSetPeerSyncAuth_usesBearerWhenTokenConfigured(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://peer/api/sync/export", nil)
	cfg := &config.Config{
		Network: config.NetworkConfig{SyncToken: "shared-secret"},
		Auth:    config.AuthCredentials{Username: "admin"},
	}
	setPeerSyncAuth(req, cfg)
	if got := req.Header.Get("Authorization"); got != "Bearer shared-secret" {
		t.Fatalf("Authorization: got %q", got)
	}
}

func TestSetPeerSyncAuth_fallsBackToBasicAuth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://peer/api/sync/export", nil)
	cfg := &config.Config{}
	cfg.Auth.Username = "admin"
	if err := cfg.Auth.SetPassword("secret"); err != nil {
		t.Fatal(err)
	}
	setPeerSyncAuth(req, cfg)
	user, pass, ok := req.BasicAuth()
	if !ok || user != "admin" || pass != "secret" {
		t.Fatalf("basic auth: ok=%v user=%q pass=%q", ok, user, pass)
	}
}
