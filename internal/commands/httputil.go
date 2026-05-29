package commands

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/keshon/commandkit"
)

// HTTPData carries the HTTP request context for command execution.
type HTTPData struct {
	W      http.ResponseWriter
	R      *http.Request
	PathID string // Set when using path param {id}
}

func getHTTPData(inv *commandkit.Invocation) *HTTPData {
	if inv == nil || inv.Data == nil {
		return nil
	}
	d, ok := inv.Data.(*HTTPData)
	if !ok {
		return nil
	}
	return d
}

func writeJSONTo(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeJSONToWriter(out io.Writer, v any) {
	json.NewEncoder(out).Encode(v)
}
