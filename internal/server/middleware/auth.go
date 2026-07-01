package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/keshon/beacon/internal/cluster"
)

const (
	SessionCookie = "uptime_session"
	sessionTTL    = 24 * time.Hour
	SessionMaxAge = int(sessionTTL / time.Second)
)

type Session struct {
	Username  string
	Created   time.Time
	CSRFToken string
}

type Auth struct {
	sessions map[string]Session
	mu       sync.RWMutex
}

func NewAuth() *Auth {
	return &Auth{
		sessions: make(map[string]Session),
	}
}

func RandomSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func (a *Auth) CreateSession(username string) (sessionID, csrfToken string, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pruneSessionsLocked()
	sid, err := RandomSessionID()
	if err != nil {
		return "", "", err
	}
	csrf, err := RandomSessionID()
	if err != nil {
		return "", "", err
	}
	a.sessions[sid] = Session{Username: username, Created: time.Now(), CSRFToken: csrf}
	return sid, csrf, nil
}

func (a *Auth) GetSession(sid string) *Session {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pruneSessionsLocked()
	s, ok := a.sessions[sid]
	if !ok {
		return nil
	}
	if time.Since(s.Created) > sessionTTL {
		delete(a.sessions, sid)
		return nil
	}
	return &s
}

func (a *Auth) DeleteSession(sid string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.sessions, sid)
}

func (a *Auth) pruneSessionsLocked() {
	now := time.Now()
	for id, s := range a.sessions {
		if now.Sub(s.Created) > sessionTTL {
			delete(a.sessions, id)
		}
	}
}

func SessionCookieSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); strings.EqualFold(proto, "https") {
		return true
	}
	return false
}

func (a *Auth) Middleware(username string, checkPassword func(user, pass string) bool, syncToken func() string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/login" || r.URL.Path == "/logout" || r.URL.Path == "/api/health" {
				next.ServeHTTP(w, r)
				return
			}
			token := ""
			if syncToken != nil {
				token = strings.TrimSpace(syncToken())
			}
			if r.URL.Path == "/api/sync/export" && token != "" {
				if cluster.SyncTokenMatches(r, token) {
					next.ServeHTTP(w, r)
					return
				}
				denyAuth(w, r)
				return
			}
			if auth := r.Header.Get("Authorization"); auth != "" && strings.HasPrefix(auth, "Basic ") {
				dec, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
				if err == nil {
					parts := strings.SplitN(string(dec), ":", 2)
					if len(parts) == 2 && subtle.ConstantTimeCompare([]byte(parts[0]), []byte(username)) == 1 &&
						checkPassword(parts[0], parts[1]) {
						next.ServeHTTP(w, r)
						return
					}
				}
			}
			cookie, err := r.Cookie(SessionCookie)
			if err != nil || cookie == nil {
				denyAuth(w, r)
				return
			}
			sess := a.GetSession(cookie.Value)
			if sess == nil {
				denyAuth(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func denyAuth(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		w.Header().Set("WWW-Authenticate", `Basic realm="Beacon"`)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}
