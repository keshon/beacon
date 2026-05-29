package web

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

const (
	csrfCookieName = "beacon_csrf"
	csrfHeaderName = "X-CSRF-Token"
)

func (a *Auth) issueCSRFCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
		Secure:   sessionCookieSecure(r),
		MaxAge:   sessionMaxAge,
	})
}

func readCSRFCookie(r *http.Request) string {
	c, err := r.Cookie(csrfCookieName)
	if err != nil || c == nil {
		return ""
	}
	return c.Value
}

func usesBasicAuth(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	return auth != "" && strings.HasPrefix(auth, "Basic ")
}

// CSRFMiddleware validates double-submit token for cookie-authenticated mutating API requests.
func (a *Auth) CSRFMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if usesBasicAuth(r) {
				next.ServeHTTP(w, r)
				return
			}
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}
			if !strings.HasPrefix(r.URL.Path, "/api/") {
				next.ServeHTTP(w, r)
				return
			}
			cookieToken := readCSRFCookie(r)
			headerToken := strings.TrimSpace(r.Header.Get(csrfHeaderName))
			if cookieToken == "" || headerToken == "" ||
				subtle.ConstantTimeCompare([]byte(cookieToken), []byte(headerToken)) != 1 {
				http.Error(w, "invalid csrf token", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
