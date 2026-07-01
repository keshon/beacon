package runner

import (
	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/storage"
)

// List returns all monitors from the store.
func List(st *storage.Store) ([]*monitor.Monitor, error) {
	return st.GetMonitors()
}

// ListRedacted returns monitors with secrets omitted (API-safe).
func ListRedacted(st *storage.Store) ([]*monitor.Monitor, error) {
	list, err := st.GetMonitors()
	if err != nil {
		return nil, err
	}
	out := make([]*monitor.Monitor, 0, len(list))
	for _, m := range list {
		out = append(out, m.Redacted())
	}
	return out, nil
}
