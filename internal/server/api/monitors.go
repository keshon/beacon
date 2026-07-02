package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/monitor/checks"
	"github.com/keshon/beacon/internal/monitor/runner"
	"github.com/keshon/beacon/internal/server/httpx"
	"github.com/keshon/beacon/internal/storage"
)

var errInvalidJSON = errors.New("invalid JSON")

type Monitors struct {
	Store *storage.Store
}

func (h *Monitors) List(w http.ResponseWriter, r *http.Request) {
	list, err := runner.ListRedacted(h.Store)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	httpx.JSON(w, list)
}

func (h *Monitors) Create(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	m, err := addMonitorFromJSON(h.Store, body)
	if err != nil {
		writeMonitorError(w, err)
		return
	}
	httpx.JSON(w, m)
}

func (h *Monitors) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	if err := runner.Delete(h.Store, id); err != nil {
		if errors.Is(err, storage.ErrMonitorNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Monitors) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	m, err := updateMonitorFromJSON(h.Store, id, body)
	if err != nil {
		writeMonitorError(w, err)
		return
	}
	httpx.JSON(w, m)
}

func (h *Monitors) Uptime(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	mon, err := h.Store.GetMonitor(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	st, err := h.Store.GetState(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if mon == nil && st == nil {
		http.NotFound(w, r)
		return
	}
	limit := 120
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			limit = n
		}
	}
	samples, err := h.Store.GetUptimeSamples(id, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	type point struct {
		Time    string `json:"time"`
		Success bool   `json:"success"`
	}
	out := make([]point, 0, len(samples))
	for _, rec := range samples {
		out = append(out, point{
			Time:    rec.Time.UTC().Format(time.RFC3339Nano),
			Success: rec.Success,
		})
	}
	httpx.JSON(w, out)
}

func (h *Monitors) UptimeBatch(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimSpace(r.URL.Query().Get("ids"))
	if raw == "" {
		http.Error(w, "missing ids", http.StatusBadRequest)
		return
	}
	ids := strings.Split(raw, ",")
	if len(ids) > 500 {
		http.Error(w, "too many ids", http.StatusBadRequest)
		return
	}
	for i := range ids {
		ids[i] = strings.TrimSpace(ids[i])
	}

	limit := 120
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			limit = n
		}
	}

	samplesByID, err := h.Store.GetUptimeSamplesBatch(ids, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type point struct {
		Time    string `json:"time"`
		Success bool   `json:"success"`
	}
	out := make(map[string][]point, len(samplesByID))
	for id, samples := range samplesByID {
		pts := make([]point, 0, len(samples))
		for _, rec := range samples {
			pts = append(pts, point{
				Time:    rec.Time.UTC().Format(time.RFC3339Nano),
				Success: rec.Success,
			})
		}
		out[id] = pts
	}
	httpx.JSON(w, out)
}

func writeMonitorError(w http.ResponseWriter, err error) {
	if errors.Is(err, errInvalidJSON) {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if errors.Is(err, storage.ErrMonitorNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	http.Error(w, err.Error(), http.StatusBadRequest)
}

func addMonitorFromJSON(st *storage.Store, body []byte) (*monitor.Monitor, error) {
	var req struct {
		Name           string                  `json:"name"`
		Type           string                  `json:"type"`
		Target         string                  `json:"target"`
		Interval       int                     `json:"interval"`
		Timeout        int                     `json:"timeout"`
		Retries        int                     `json:"retries"`
		HTTP           *checks.HTTPOptions     `json:"http,omitempty"`
		NotifyOverride *monitor.NotifyOverride `json:"notify_override"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, errInvalidJSON
	}
	m, err := runner.Add(st, runner.AddInput{
		Name:           req.Name,
		Type:           req.Type,
		Target:         req.Target,
		IntervalSec:    req.Interval,
		TimeoutSec:     req.Timeout,
		Retries:        req.Retries,
		HTTP:           req.HTTP,
		NotifyOverride: req.NotifyOverride,
	})
	if err != nil {
		return nil, err
	}
	return m.Redacted(), nil
}

func updateMonitorFromJSON(st *storage.Store, id string, body []byte) (*monitor.Monitor, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, errInvalidJSON
	}
	patch := runner.UpdatePatch{}
	if v, ok := raw["enabled"]; ok {
		var b bool
		if err := json.Unmarshal(v, &b); err != nil {
			return nil, errInvalidJSON
		}
		patch.Enabled = &b
	}
	if v, ok := raw["name"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return nil, errInvalidJSON
		}
		patch.Name = &s
	}
	if v, ok := raw["type"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return nil, errInvalidJSON
		}
		patch.Type = &s
	}
	if v, ok := raw["target"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return nil, errInvalidJSON
		}
		patch.Target = &s
	}
	if v, ok := raw["interval"]; ok {
		var n int
		if err := json.Unmarshal(v, &n); err != nil {
			return nil, errInvalidJSON
		}
		patch.IntervalSec = &n
	}
	if v, ok := raw["http"]; ok {
		var h checks.HTTPOptions
		if err := json.Unmarshal(v, &h); err != nil {
			return nil, errInvalidJSON
		}
		patch.HTTP = &h
	}
	if v, ok := raw["notify_override"]; ok {
		trimmed := strings.TrimSpace(string(v))
		if trimmed != "" && trimmed != "null" && trimmed != "{}" {
			var no monitor.NotifyOverride
			if err := json.Unmarshal(v, &no); err != nil {
				return nil, errInvalidJSON
			}
			patch.NotifyOverride = &no
		}
	}
	m, err := runner.Update(st, id, patch)
	if err != nil {
		return nil, err
	}
	return m.Redacted(), nil
}
