package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/keshon/beacon/internal/cluster"
	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor/scheduler"
	"github.com/keshon/beacon/internal/server/httpx"
	"github.com/keshon/beacon/internal/storage"
)

type Config struct {
	Store   *storage.Store
	Cfg     *config.Live
	Cluster *cluster.Runtime
}

func (h *Config) Get(w http.ResponseWriter, r *http.Request) {
	pub, err := getSettingsConfig(h.Store)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	pub.RequiresRestart = false
	httpx.JSON(w, pub)
}

func (h *Config) Set(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	before := h.Cfg.Load()
	next, pub, err := applyConfigPatch(h.Store, body)
	if err != nil {
		if err == errInvalidJSON {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	next.Auth.CarryPlainPassword(before.Auth)
	h.Cfg.Store(next)
	if h.Cluster != nil {
		h.Cluster.NotifyConfigChange()
	}
	pub.RequiresRestart = configNeedsRestart(before, next)
	httpx.JSON(w, pub)
}

func configNeedsRestart(before, after *config.Config) bool {
	if before == nil || after == nil {
		return false
	}
	return before.Listen != after.Listen || before.Workers != after.Workers
}

func getSettingsConfig(st *storage.Store) (config.PublicConfig, error) {
	var cfg config.Config
	ok, err := st.GetConfig(&cfg)
	if err != nil {
		return config.PublicConfig{}, err
	}
	if !ok {
		cfg = *config.Default()
	}
	cfg.Normalize()
	return cfg.ToSettings(), nil
}

// applyConfigPatch merges the incoming patch over the persisted config and
// returns the new runtime snapshot; the caller publishes it via Live.Store.
func applyConfigPatch(st *storage.Store, body []byte) (*config.Config, config.PublicConfig, error) {
	var incoming config.Config
	if err := json.Unmarshal(body, &incoming); err != nil {
		return nil, config.PublicConfig{}, errInvalidJSON
	}
	// Which sections the patch mentioned — see config.Sections. Without this a
	// screen that saves only its own part would zero everyone else's.
	sec, err := config.ParseSections(body)
	if err != nil {
		return nil, config.PublicConfig{}, errInvalidJSON
	}

	var existing config.Config
	ok, err := st.GetConfig(&existing)
	if err != nil {
		return nil, config.PublicConfig{}, err
	}
	if !ok {
		existing = *config.Default()
	}
	config.ApplyNonSecret(&existing, &incoming, sec)
	if err := config.MergeSecrets(&existing, &incoming, sec); err != nil {
		return nil, config.PublicConfig{}, err
	}
	existing.Normalize()
	if err := existing.Auth.EnsureHashed(); err != nil {
		return nil, config.PublicConfig{}, err
	}
	if err := st.SetConfig(&existing); err != nil {
		return nil, config.PublicConfig{}, err
	}
	return &existing, existing.ToSettings(), nil
}

type State struct {
	Store     *storage.Store
	Scheduler *scheduler.Scheduler
}

func (h *State) Get(w http.ResponseWriter, r *http.Request) {
	st, err := h.Store.GetAllState()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	httpx.JSON(w, st)
}

func (h *State) CheckRecords(w http.ResponseWriter, r *http.Request) {
	limit := parseRecordLimit(r.URL.Query().Get("limit"))
	records, err := h.Store.GetCheckRecords(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	httpx.JSON(w, records)
}

func (h *State) Health(w http.ResponseWriter, r *http.Request) {
	type health struct {
		Status        string `json:"status"`
		StoreOK       bool   `json:"store_ok"`
		DroppedChecks uint64 `json:"dropped_checks"`
	}
	hlth := health{Status: "ok", StoreOK: true}
	if err := h.Store.Ping(); err != nil {
		hlth.Status = "degraded"
		hlth.StoreOK = false
	}
	if h.Scheduler != nil {
		hlth.DroppedChecks = h.Scheduler.DroppedChecks()
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(hlth)
}

func parseRecordLimit(s string) int {
	if s == "" {
		return 100
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 100
	}
	return n
}
