package httpx

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/flosch/pongo2/v6"
)

var (
	tplMu    sync.RWMutex
	tplCache = map[string]*pongo2.Template{}
	tplStamp = map[string]time.Time{}
)

// devReload re-reads a template when its file changed. Only under
// BEACON_DEV=1: in production templates do not change, and an extra
// stat per request buys nothing there.
var devReload = os.Getenv("BEACON_DEV") == "1"

func JSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func Render(w http.ResponseWriter, tplDir, name string, ctx pongo2.Context) error {
	fullPath := filepath.Join(tplDir, name)

	// Development bypasses the cache entirely. Watching the file's own
	// mtime is not enough: pongo2 compiles an include into its parent, so
	// editing an included fragment does not invalidate the parent.
	var mod time.Time
	stale := devReload

	tplMu.RLock()
	tpl := tplCache[fullPath]
	tplMu.RUnlock()

	var err error
	if tpl == nil || stale {
		tplMu.Lock()
		tpl = tplCache[fullPath]
		if tpl == nil || stale {
			tpl, err = pongo2.FromFile(fullPath)
			if err != nil {
				tplMu.Unlock()
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return err
			}
			tplCache[fullPath] = tpl
			tplStamp[fullPath] = mod
		}
		tplMu.Unlock()
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}
	return tpl.ExecuteWriter(ctx, w)
}
