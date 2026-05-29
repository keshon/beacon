package web

import (
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	sessionCookie   = "uptime_session"
	sessionTTL      = 24 * time.Hour
	sessionMaxAge   = int(sessionTTL / time.Second)
)

type Session struct {
	Username string
	Created  time.Time
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

func (a *Auth) CreateSession(username string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pruneSessionsLocked()
	sid, err := randomID()
	if err != nil {
		return "", err
	}
	a.sessions[sid] = Session{Username: username, Created: time.Now()}
	return sid, nil
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

func sessionCookieSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); strings.EqualFold(proto, "https") {
		return true
	}
	return false
}

func (a *Auth) Middleware(username string, checkPassword func(user, pass string) bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/login" || r.URL.Path == "/logout" || r.URL.Path == "/api/health" {
				next.ServeHTTP(w, r)
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
			cookie, err := r.Cookie(sessionCookie)
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
