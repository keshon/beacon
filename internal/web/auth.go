package web

import (
	"encoding/base64"
	"net/http"
	"strings"
	"sync"
	"time"
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

func (a *Auth) CreateSession(username string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	sid := randomID()
	a.sessions[sid] = Session{Username: username, Created: time.Now()}
	return sid
}

func (a *Auth) GetSession(sid string) *Session {
	a.mu.RLock()
	defer a.mu.RUnlock()
	s, ok := a.sessions[sid]
	if !ok {
		return nil
	}
	return &s
}

func (a *Auth) DeleteSession(sid string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.sessions, sid)
}

const sessionCookie = "uptime_session"

func (a *Auth) Middleware(username, password string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/login" || r.URL.Path == "/logout" || r.URL.Path == "/api/health" {
				next.ServeHTTP(w, r)
				return
			}
			// Allow Basic auth for API sync
			if auth := r.Header.Get("Authorization"); auth != "" && strings.HasPrefix(auth, "Basic ") {
				dec, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
				if err == nil {
					parts := strings.SplitN(string(dec), ":", 2)
					if len(parts) == 2 && parts[0] == username && parts[1] == password {
						next.ServeHTTP(w, r)
						return
					}
				}
			}
			cookie, err := r.Cookie(sessionCookie)
			if err != nil || cookie == nil {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
			sess := a.GetSession(cookie.Value)
			if sess == nil {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
