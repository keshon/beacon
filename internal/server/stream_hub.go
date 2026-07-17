package server

import (
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/storage"
)

const defaultClientBuf = 64

// CheckStreamHub fans out check results to SSE subscribers.
type CheckStreamHub struct {
	mu      sync.Mutex
	clients map[chan []byte]*sync.Once
}

// NewCheckStreamHub creates a new SSE hub for check results.
func NewCheckStreamHub() *CheckStreamHub {
	return &CheckStreamHub{clients: make(map[chan []byte]*sync.Once)}
}

// closeClient removes and closes a client channel. Safe to call from both
// broadcast (slow client) and unregister (handler exit) — close runs once.
func (h *CheckStreamHub) closeClient(c chan []byte) {
	h.mu.Lock()
	once := h.clients[c]
	delete(h.clients, c)
	h.mu.Unlock()
	if once != nil {
		once.Do(func() { close(c) })
	}
}

func (h *CheckStreamHub) Register(buf int) (ch <-chan []byte, unregister func()) {
	if buf < defaultClientBuf {
		buf = defaultClientBuf
	}
	c := make(chan []byte, buf)
	h.mu.Lock()
	h.clients[c] = new(sync.Once)
	h.mu.Unlock()
	return c, func() { h.closeClient(c) }
}

func (h *CheckStreamHub) BroadcastCheck(rec storage.CheckRecord, st *monitor.MonitorState) {
	h.broadcast(formatCheckEvent(rec.MonitorID, rec.Success, rec.Time, st))
}

func formatStateEvent(monitorID string, st *monitor.MonitorState) []byte {
	if st == nil {
		return nil
	}
	success := st.Status == monitor.StatusUp
	t := st.LastCheck
	if t.IsZero() {
		t = time.Now()
	}
	return formatCheckEvent(monitorID, success, t, st)
}

func formatCheckEvent(monitorID string, success bool, at time.Time, st *monitor.MonitorState) []byte {
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
		MonitorID: monitorID,
		Success:   success,
		Time:      at.UTC().Format(time.RFC3339Nano),
		Status:    status,
		LatencyMs: latencyMs,
		LastCheck: lastCheck,
	})
	if err != nil {
		return nil
	}
	line := make([]byte, 0, len(payload)+8)
	line = append(line, "data: "...)
	line = append(line, payload...)
	line = append(line, '\n', '\n')
	return line
}

func (h *CheckStreamHub) broadcast(line []byte) {
	if len(line) == 0 {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for c, once := range h.clients {
		select {
		case c <- line:
		default:
			once.Do(func() { close(c) })
			delete(h.clients, c)
		}
	}
}
