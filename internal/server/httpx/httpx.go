package httpx

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/flosch/pongo2/v6"
)

var (
	tplMu    sync.RWMutex
	tplCache = map[string]*pongo2.Template{}
)

func JSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func Render(w http.ResponseWriter, tplDir, name string, ctx pongo2.Context) error {
	fullPath := filepath.Join(tplDir, name)

	tplMu.RLock()
	tpl := tplCache[fullPath]
	tplMu.RUnlock()

	var err error
	if tpl == nil {
		tplMu.Lock()
		tpl = tplCache[fullPath]
		if tpl == nil {
			tpl, err = pongo2.FromFile(fullPath)
			if err != nil {
				tplMu.Unlock()
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return err
			}
			tplCache[fullPath] = tpl
		}
		tplMu.Unlock()
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}
	return tpl.ExecuteWriter(ctx, w)
}
