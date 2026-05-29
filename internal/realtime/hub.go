package realtime

import (
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/store"
)

// Hub fans out check results to SSE subscribers.
type Hub struct {
	mu      sync.Mutex
	clients map[chan []byte]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: make(map[chan []byte]struct{})}
}

// Register adds a subscriber. The returned channel receives complete SSE data lines
// (including "data: " prefix and trailing newlines). Call unregister when the client disconnects.
func (h *Hub) Register(buf int) (ch <-chan []byte, unregister func()) {
	if buf < 4 {
		buf = 4
	}
	c := make(chan []byte, buf)
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	return c, func() {
		h.mu.Lock()
		delete(h.clients, c)
		h.mu.Unlock()
		close(c)
	}
}

// BroadcastCheck notifies subscribers of a completed check (non-blocking per client).
// st is the persisted monitor state after the check (may be nil only if caller passes nil).
func (h *Hub) BroadcastCheck(rec store.CheckRecord, st *monitor.MonitorState) {
	status := monitor.StatusUnknown
	latencyMs := "—"
	lastCheck := "—"
	if st != nil {
		status = st.Status
		if st.Latency > 0 {
			latencyMs = strconv.FormatInt(st.Latency.Milliseconds(), 10) + "ms"
		}
		if !st.LastCheck.IsZero() {
			lastCheck = st.LastCheck.Format("15:04:05")
		}
	}
	type wire struct {
		MonitorID string `json:"monitor_id"`
		Success   bool   `json:"success"`
		Time      string `json:"time"`
		Status    string `json:"status"`
		LatencyMs string `json:"latency_ms"`
		LastCheck string `json:"last_check"`
	}
	payload, err := json.Marshal(wire{
		MonitorID: rec.MonitorID,
		Success:   rec.Success,
		Time:      rec.Time.UTC().Format(time.RFC3339Nano),
		Status:    status,
		LatencyMs: latencyMs,
		LastCheck: lastCheck,
	})
	if err != nil {
		return
	}
	line := make([]byte, 0, len(payload)+8)
	line = append(line, "data: "...)
	line = append(line, payload...)
	line = append(line, '\n', '\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		select {
		case c <- line:
		default:
			// Drop for slow readers; client will catch up on next poll or reconnect.
		}
	}
}
