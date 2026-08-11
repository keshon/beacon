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

// devReload — перечитывать шаблон, если файл изменился. Только при
// BEACON_DEV=1: в бою шаблоны не меняются, и лишний stat на каждый запрос
// там не нужен.
var devReload = os.Getenv("BEACON_DEV") == "1"

func JSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func Render(w http.ResponseWriter, tplDir, name string, ctx pongo2.Context) error {
	fullPath := filepath.Join(tplDir, name)

	// В разработке кэш не используется вовсе. Следить за временем правки
	// самого файла мало: pongo2 вкомпилирует include в родителя, и правка
	// подключённого куска родителя не протухает.
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
