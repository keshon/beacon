package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/keshon/beacon/internal/monitor/scheduler"
	"github.com/keshon/beacon/internal/server/httpx"
	"github.com/keshon/beacon/internal/storage"
)

// What a person can say back.
//
// Every screen in this application could tell you something and none of them
// could be told anything: the whole vocabulary was edit, pause and delete. So
// the answer to "it is down, I just fixed it" was to wait out the interval, and
// the answer to "it is down because I am working on it" was to endure the
// alerts or disable the monitor and lose the history.
//
// Three verbs close that: check it now, keep quiet about it for a while, and
// note that it is known. None of them is clever; the absence of all three was.
type Actions struct {
	Store     *storage.Store
	Scheduler *scheduler.Scheduler
}

// CheckNow runs a check immediately, through the ordinary queue.
func (h *Actions) CheckNow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if ok, err := h.Store.MonitorExists(id); err != nil || !ok {
		http.NotFound(w, r)
		return
	}
	switch err := h.Scheduler.CheckNow(id); err {
	case nil:
		httpx.JSON(w, map[string]any{"queued": true})
	case scheduler.ErrCheckInFlight:
		// Already happening is not a failure. Saying "running" is the truth
		// and asks for nothing.
		httpx.JSON(w, map[string]any{"queued": false, "reason": "already running"})
	default:
		http.Error(w, "check queue is busy, try again", http.StatusServiceUnavailable)
	}
}

type muteRequest struct {
	// Minutes of silence. Zero lifts the mute — the same control both ways,
	// because a mute you cannot cancel is a trap.
	Minutes int    `json:"minutes"`
	Note    string `json:"note"`
}

// Mute silences a monitor's alerts for a while. Checks keep running.
func (h *Actions) Mute(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if ok, err := h.Store.MonitorExists(id); err != nil || !ok {
		http.NotFound(w, r)
		return
	}
	var req muteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	// A day is the ceiling on purpose: silence that outlives the shift that
	// asked for it is how an outage goes unnoticed for a week.
	if req.Minutes > 24*60 {
		req.Minutes = 24 * 60
	}

	var until time.Time
	if req.Minutes > 0 {
		until = time.Now().Add(time.Duration(req.Minutes) * time.Minute)
	}
	if err := h.Store.SetMute(id, until, req.Note); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	httpx.JSON(w, map[string]any{"muted": req.Minutes > 0, "until": until})
}

type ackRequest struct {
	Note string `json:"note"`
}

// Acknowledge marks the running incident as seen, with a note for whoever
// looks next — which is often the same person, hours later, without the
// context they have right now.
func (h *Actions) Acknowledge(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if ok, err := h.Store.MonitorExists(id); err != nil || !ok {
		http.NotFound(w, r)
		return
	}
	var req ackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if err := h.Store.AcknowledgeIncident(id, "web", req.Note); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	httpx.JSON(w, map[string]any{"acknowledged": true})
}
