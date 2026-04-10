package sync

import (
	"time"

	"github.com/keshon/beacon/internal/monitor"
)

// ExportPayload is returned by GET /api/sync/export.
type ExportPayload struct {
	NodeID   string                           `json:"node_id"`
	Monitors []*monitor.Monitor               `json:"monitors"`
	State    map[string]*monitor.MonitorState `json:"state"`
	Time     time.Time                        `json:"time"`
}
