package httpx

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"github.com/flosch/pongo2/v6"
)

func JSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func Render(w http.ResponseWriter, tplDir, name string, ctx pongo2.Context) error {
	tpl, err := pongo2.FromFile(filepath.Join(tplDir, name))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}
	return tpl.ExecuteWriter(ctx, w)
}
