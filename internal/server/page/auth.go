package page

import (
	"net/http"

	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/server/httpx"
	"github.com/keshon/beacon/internal/server/middleware"

	"github.com/flosch/pongo2/v6"
)

type Auth struct {
	Auth   *middleware.Auth
	Cfg    *config.Config
	TplDir string
}

func (h *Auth) LoginForm(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(middleware.SessionCookie)
	if cookie != nil && h.Auth.GetSession(cookie.Value) != nil {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
		return
	}
	_ = httpx.Render(w, h.TplDir, "login.html", pongo2.Context{})
}

func (h *Auth) Login(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	user := r.FormValue("username")
	pass := r.FormValue("password")
	if user != h.Cfg.Auth.Username || !h.Cfg.Auth.CheckPassword(pass) {
		_ = httpx.Render(w, h.TplDir, "login.html", pongo2.Context{"error": "Invalid credentials"})
		return
	}
	sid, csrf, err := h.Auth.CreateSession(user)
	if err != nil {
		http.Error(w, "login failed", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookie,
		Value:    sid,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   middleware.SessionCookieSecure(r),
		MaxAge:   middleware.SessionMaxAge,
	})
	h.Auth.IssueCSRFCookie(w, r, csrf)
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func (h *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	if c, _ := r.Cookie(middleware.SessionCookie); c != nil {
		h.Auth.DeleteSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: middleware.SessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: middleware.SessionCookieSecure(r)})
	http.SetCookie(w, &http.Cookie{Name: middleware.CSRFCookieName, Value: "", Path: "/", MaxAge: -1, Secure: middleware.SessionCookieSecure(r)})
	http.Redirect(w, r, "/login", http.StatusFound)
}
