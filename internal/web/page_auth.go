package web

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/flosch/pongo2/v6"
)

func randomSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func (s *Server) pageLoginForm(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(sessionCookie)
	if cookie != nil && s.auth.GetSession(cookie.Value) != nil {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
		return
	}
	s.render(w, "login.html", pongo2.Context{})
}

func (s *Server) pageLogin(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	user := r.FormValue("username")
	pass := r.FormValue("password")
	if user != s.cfg.Auth.Username || !s.cfg.Auth.CheckPassword(pass) {
		s.render(w, "login.html", pongo2.Context{"error": "Invalid credentials"})
		return
	}
	sid, err := s.auth.CreateSession(user)
	if err != nil {
		http.Error(w, "login failed", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    sid,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   sessionCookieSecure(r),
		MaxAge:   sessionMaxAge,
	})
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func (s *Server) pageLogout(w http.ResponseWriter, r *http.Request) {
	if c, _ := r.Cookie(sessionCookie); c != nil {
		s.auth.DeleteSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: sessionCookieSecure(r)})
	http.Redirect(w, r, "/login", http.StatusFound)
}
