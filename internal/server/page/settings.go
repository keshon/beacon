package page

import (
	"net/http"

	"github.com/keshon/beacon/internal/server/httpx"

	"github.com/flosch/pongo2/v6"
)

type Settings struct {
	TplDir string
}

func (h *Settings) Serve(w http.ResponseWriter, r *http.Request) {
	_ = httpx.Render(w, h.TplDir, "settings/settings.html", pongo2.Context{
		"version":    buildVersion(),
		"nav_active": "settings",
	})
}
