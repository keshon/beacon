package page

import (
	"net/http"
)

type Monitors struct {
	TplDir string
}

func (h *Monitors) Serve(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}
