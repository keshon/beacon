package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSyncTokenFromRequest(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/sync/export", nil)
	r.Header.Set("Authorization", "Bearer my-token")
	if got := SyncTokenFromRequest(r); got != "my-token" {
		t.Fatalf("Bearer: got %q", got)
	}

	r = httptest.NewRequest(http.MethodGet, "/api/sync/export", nil)
	r.Header.Set("X-Beacon-Sync-Token", "header-token")
	if got := SyncTokenFromRequest(r); got != "header-token" {
		t.Fatalf("header: got %q", got)
	}
}

func TestSyncTokenMatches(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/sync/export", nil)
	r.Header.Set("Authorization", "Bearer correct")
	if !SyncTokenMatches(r, "correct") {
		t.Fatal("expected match")
	}
	if SyncTokenMatches(r, "wrong") {
		t.Fatal("expected mismatch")
	}
}

func TestMiddleware_syncExportRequiresTokenWhenConfigured(t *testing.T) {
	auth := NewAuth()
	called := false
	h := auth.Middleware("admin", func(user, pass string) bool {
		return user == "admin" && pass == "admin"
	}, func() string { return "peer-secret" })(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("valid bearer", func(t *testing.T) {
		called = false
		r := httptest.NewRequest(http.MethodGet, "/api/sync/export", nil)
		r.Header.Set("Authorization", "Bearer peer-secret")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusOK || !called {
			t.Fatalf("valid token: status=%d called=%v", w.Code, called)
		}
	})

	t.Run("invalid bearer", func(t *testing.T) {
		called = false
		r := httptest.NewRequest(http.MethodGet, "/api/sync/export", nil)
		r.Header.Set("Authorization", "Bearer wrong")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusUnauthorized || called {
			t.Fatalf("invalid token: status=%d called=%v", w.Code, called)
		}
	})

	t.Run("legacy basic when token unset", func(t *testing.T) {
		hLegacy := auth.Middleware("admin", func(user, pass string) bool {
			return user == "admin" && pass == "admin"
		}, func() string { return "" })(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		r := httptest.NewRequest(http.MethodGet, "/api/sync/export", nil)
		r.SetBasicAuth("admin", "admin")
		w := httptest.NewRecorder()
		hLegacy.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("legacy basic: status=%d", w.Code)
		}
	})
}
