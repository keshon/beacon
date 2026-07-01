package page

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/server/httpx"
	"github.com/keshon/beacon/internal/storage"

	"github.com/flosch/pongo2/v6"
)

type Monitors struct {
	Store  *storage.Store
	TplDir string
}

func (h *Monitors) Serve(w http.ResponseWriter, r *http.Request) {
	monitors, err := h.Store.GetMonitors()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	type monitorRow struct {
		*monitor.Monitor
		IntervalSec int
		NotifyJSON  string
		HTTPJSON    string
	}
	rows := make([]monitorRow, 0, len(monitors))
	for _, m := range monitors {
		sec := 0
		if m.Interval > 0 {
			sec = int(m.Interval / time.Second)
		}
		notifyJSON := "{}"
		if m.NotifyOverride != nil {
			buf, _ := json.Marshal(m.NotifyOverride)
			notifyJSON = string(buf)
		}
		httpJSON := "{}"
		if m.HTTP != nil {
			if buf, err := json.Marshal(m.HTTP.Redacted()); err == nil {
				httpJSON = string(buf)
			}
		}
		rows = append(rows, monitorRow{
			Monitor:     m.Redacted(),
			IntervalSec: sec,
			NotifyJSON:  notifyJSON,
			HTTPJSON:    httpJSON,
		})
	}
	_ = httpx.Render(w, h.TplDir, "monitors/monitors.html", pongo2.Context{
		"version":    buildVersion(),
		"nav_active": "monitors",
		"monitors":   rows,
	})
}
