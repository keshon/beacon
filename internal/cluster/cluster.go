package cluster

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/store"
)

// Runtime owns peer sync and adoption. It implements scheduler.MonitorSource when enabled.
type Runtime struct {
	store  *store.Store
	cfg    *config.Config
	client *http.Client

	adoptedMu sync.RWMutex
	adopted   map[string]*monitor.Monitor

	syncNow chan struct{}
}

// New creates a cluster runtime. Always non-nil; peer sync runs when network.enabled.
func New(st *store.Store, cfg *config.Config) *Runtime {
	if cfg == nil {
		cfg = config.Default()
	}
	return &Runtime{
		store:   st,
		cfg:     cfg,
		client:  &http.Client{Timeout: 15 * time.Second},
		adopted: make(map[string]*monitor.Monitor),
		syncNow: make(chan struct{}, 1),
	}
}

// Enabled reports whether peer sync is active.
func (rt *Runtime) Enabled() bool {
	return rt != nil && rt.cfg != nil && rt.cfg.Network.Enabled
}

// NotifyConfigChange triggers an immediate peer sync (e.g. after peers list edit).
func (rt *Runtime) NotifyConfigChange() {
	if rt == nil {
		return
	}
	select {
	case rt.syncNow <- struct{}{}:
	default:
	}
}

// Run starts the background peer sync loop.
func (rt *Runtime) Run(ctx context.Context) {
	if rt == nil {
		return
	}
	rt.runSync(ctx)
}

func (rt *Runtime) refreshAdopted(peerData map[string]*store.PeerData, now time.Time) {
	next := make(map[string]*monitor.Monitor)
	for _, am := range adoptedMonitors(rt.cfg, peerData, now) {
		if am.Monitor != nil {
			next[am.Monitor.ID] = am.Monitor
			if st, ok := peerData[am.OwnerNodeID]; ok && st != nil {
				if peerSt, ok := st.State[am.Monitor.ID]; ok && peerSt != nil {
					local, _ := rt.store.GetState(am.Monitor.ID)
					merged := mergeMonitorState(local, peerSt)
					if merged != nil && (local == nil || merged.LastCheck.After(local.LastCheck)) {
						_ = rt.store.SetState(merged)
					}
				}
			}
		}
	}
	rt.adoptedMu.Lock()
	rt.adopted = next
	rt.adoptedMu.Unlock()
}

// List implements scheduler.MonitorSource.
func (rt *Runtime) List(ctx context.Context) ([]*monitor.Monitor, error) {
	_ = ctx
	if !rt.Enabled() {
		return rt.store.GetMonitors()
	}
	own, err := rt.store.GetMonitors()
	if err != nil {
		return nil, err
	}
	peerData, err := rt.store.GetAllPeerData()
	if err != nil {
		return own, err
	}
	now := time.Now()
	rt.refreshAdopted(peerData, now)

	rt.adoptedMu.RLock()
	adopted := rt.adopted
	rt.adoptedMu.RUnlock()

	byID := make(map[string]*monitor.Monitor, len(own)+len(adopted))
	for _, m := range own {
		byID[m.ID] = m
	}
	for id, m := range adopted {
		if existing, ok := byID[id]; ok {
			log.Printf("[cluster] monitor ID collision %s: keeping local over adopted", id)
			_ = existing
			continue
		}
		byID[id] = m
	}
	list := make([]*monitor.Monitor, 0, len(byID))
	for _, m := range byID {
		list = append(list, m)
	}
	return list, nil
}

// Resolve implements scheduler.MonitorSource.
func (rt *Runtime) Resolve(id string) (*monitor.Monitor, error) {
	if !rt.Enabled() {
		return rt.store.GetMonitor(id)
	}
	m, err := rt.store.GetMonitor(id)
	if err != nil {
		return nil, err
	}
	if m != nil {
		return m, nil
	}
	rt.adoptedMu.RLock()
	am, ok := rt.adopted[id]
	rt.adoptedMu.RUnlock()
	if ok {
		return am, nil
	}
	peerData, err := rt.store.GetAllPeerData()
	if err != nil {
		return nil, err
	}
	rt.refreshAdopted(peerData, time.Now())
	rt.adoptedMu.RLock()
	am = rt.adopted[id]
	rt.adoptedMu.RUnlock()
	return am, nil
}

// RequireLocalMonitor implements scheduler.MonitorSource.
func (rt *Runtime) RequireLocalMonitor(id string) bool {
	if !rt.Enabled() {
		return true
	}
	m, _ := rt.store.GetMonitor(id)
	return m != nil
}

// ExportView builds the sync export monitors and state.
func (rt *Runtime) ExportView() (ExportView, error) {
	return buildExportView(rt.cfg, rt.store)
}

// HandleExport serves GET /api/sync/export.
func (rt *Runtime) HandleExport(w http.ResponseWriter, r *http.Request) {
	if !rt.Enabled() {
		http.Error(w, "network not enabled", http.StatusServiceUnavailable)
		return
	}
	if rt.cfg.Network.NodeID == "" {
		http.Error(w, "network not configured", http.StatusServiceUnavailable)
		return
	}
	view, err := rt.ExportView()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	payload := ExportPayload{
		NodeID:   rt.cfg.Network.NodeID,
		Monitors: view.Monitors,
		State:    view.State,
		Time:     time.Now(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}

// DashboardRow is one monitor line on the dashboard (local, live peer, or adopted).
type DashboardRow struct {
	Monitor      *monitor.Monitor
	State        *monitor.MonitorState
	LatencyMs    string
	LastCheck    string
	Status       string
	SourceLabel  string
	SourceNodeID string
	IsPeer       bool
	Adopted      bool
}

// NetworkNode is one row in the network status panel.
type NetworkNode struct {
	NodeID        string   `json:"node_id"`
	NodeIDShort   string   `json:"node_id_short"`
	URL           string   `json:"url"`
	Status        string   `json:"status"`
	LastSeen      string   `json:"last_seen,omitempty"`
	MonitorsCount int      `json:"monitors_count"`
	LastError     string   `json:"last_error,omitempty"`
	SyncWarnings  []string `json:"sync_warnings,omitempty"`
}

// DashboardRows returns local monitors plus peer and adopted rows for the dashboard.
func (rt *Runtime) DashboardRows(localState map[string]*monitor.MonitorState, ownMonitors []*monitor.Monitor) ([]DashboardRow, error) {
	var rows []DashboardRow
	for _, m := range ownMonitors {
		st := localState[m.ID]
		row := DashboardRow{Monitor: m, State: st, Status: "unknown", SourceLabel: "This node"}
		fillRowMetrics(&row)
		rows = append(rows, row)
	}
	if !rt.Enabled() {
		return rows, nil
	}
	peerData, err := rt.store.GetAllPeerData()
	if err != nil {
		return rows, err
	}
	deadTimeout := time.Duration(rt.cfg.Network.DeadTimeout) * time.Second
	now := time.Now()
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		seen[row.Monitor.ID] = struct{}{}
	}
	for _, pd := range peerData {
		sourceLabel := peerDisplayName(pd.PeerURL)
		if !peerLive(pd, deadTimeout, now) {
			continue
		}
		for _, m := range pd.Monitors {
			if m == nil {
				continue
			}
			if _, ok := seen[m.ID]; ok {
				continue
			}
			st := pd.State[m.ID]
			if st == nil {
				st = localState[m.ID]
			}
			row := DashboardRow{
				Monitor: m, State: st, SourceLabel: "Peer: " + sourceLabel,
				SourceNodeID: pd.NodeID, IsPeer: true,
			}
			fillRowMetrics(&row)
			rows = append(rows, row)
			seen[m.ID] = struct{}{}
		}
	}
	for _, am := range adoptedMonitors(rt.cfg, peerData, now) {
		if am.Monitor == nil {
			continue
		}
		if _, ok := seen[am.Monitor.ID]; ok {
			continue
		}
		st := localState[am.Monitor.ID]
		if st == nil && peerData[am.OwnerNodeID] != nil {
			st = peerData[am.OwnerNodeID].State[am.Monitor.ID]
		}
		row := DashboardRow{
			Monitor: am.Monitor, State: st,
			SourceLabel: "Adopted: " + peerDisplayName(am.OwnerLabel),
			SourceNodeID: am.OwnerNodeID, IsPeer: true, Adopted: true,
		}
		fillRowMetrics(&row)
		rows = append(rows, row)
		seen[am.Monitor.ID] = struct{}{}
	}
	return rows, nil
}

func fillRowMetrics(row *DashboardRow) {
	row.Status = "unknown"
	if row.State != nil {
		row.Status = row.State.Status
		if row.State.Latency > 0 {
			row.LatencyMs = strconv.FormatInt(row.State.Latency.Milliseconds(), 10) + "ms"
		}
		if !row.State.LastCheck.IsZero() {
			row.LastCheck = row.State.LastCheck.Format("15:04:05")
		}
	}
	if row.LatencyMs == "" {
		row.LatencyMs = "—"
	}
	if row.LastCheck == "" {
		row.LastCheck = "—"
	}
}

// NetworkNodes builds the network status node list.
func (rt *Runtime) NetworkNodes() ([]NetworkNode, error) {
	if !rt.Enabled() {
		return []NetworkNode{}, nil
	}
	var nodes []NetworkNode
	deadTimeout := time.Duration(rt.cfg.Network.DeadTimeout) * time.Second
	peerData, _ := rt.store.GetAllPeerData()
	ownMonitors, _ := rt.store.GetMonitors()

	nodes = append(nodes, NetworkNode{
		NodeID:        rt.cfg.Network.NodeID,
		NodeIDShort:   truncateNodeID(rt.cfg.Network.NodeID, 8),
		URL:           rt.cfg.Network.SelfURL,
		Status:        "self",
		MonitorsCount: len(ownMonitors),
	})

	peerURLToData := make(map[string]*store.PeerData)
	for _, pd := range peerData {
		key := strings.TrimSuffix(pd.PeerURL, "/")
		if key == "" {
			key = pd.NodeID
		}
		peerURLToData[key] = pd
	}

	for _, peerURL := range rt.cfg.Network.Peers {
		if peerURL == "" {
			continue
		}
		trimmed := strings.TrimSuffix(peerURL, "/")
		if trimmed == strings.TrimSuffix(rt.cfg.Network.SelfURL, "/") {
			continue
		}
		pd := peerURLToData[trimmed]
		if pd == nil {
			nodes = append(nodes, NetworkNode{
				NodeIDShort: "—",
				URL:         peerURL,
				Status:      "unknown",
				LastSeen:    "—",
			})
			continue
		}
		status := "live"
		if time.Since(pd.LastSeen) >= deadTimeout {
			status = "dead"
		}
		lastSeen := "—"
		if !pd.LastSeen.IsZero() {
			lastSeen = formatTimeAgo(pd.LastSeen)
		}
		nodes = append(nodes, NetworkNode{
			NodeID:        pd.NodeID,
			NodeIDShort:   truncateNodeID(pd.NodeID, 8),
			URL:           pd.PeerURL,
			Status:        status,
			LastSeen:      lastSeen,
			MonitorsCount: len(pd.Monitors),
			LastError:     pd.LastError,
			SyncWarnings:  pd.SyncWarnings,
		})
	}
	return nodes, nil
}

// HandleNetworkStatus serves GET /api/network/status.
func (rt *Runtime) HandleNetworkStatus(w http.ResponseWriter, r *http.Request) {
	if !rt.Enabled() {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"nodes": []NetworkNode{}})
		return
	}
	nodes, err := rt.NetworkNodes()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"nodes": nodes})
}

func peerDisplayName(url string) string {
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimSuffix(url, "/")
	if url == "" {
		return "peer"
	}
	return url
}

func truncateNodeID(id string, n int) string {
	if len(id) <= n {
		return id
	}
	return id[:n] + "..."
}

func formatTimeAgo(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		m := int(d.Minutes())
		if m == 1 {
			return "1 min ago"
		}
		return strconv.Itoa(m) + " min ago"
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return strconv.Itoa(h) + " hours ago"
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day ago"
	}
	return strconv.Itoa(days) + " days ago"
}
