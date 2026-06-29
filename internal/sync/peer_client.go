package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/network"
	"github.com/keshon/beacon/internal/store"
)

// PeerSyncClient periodically fetches data from peer nodes.
type PeerSyncClient struct {
	store  *store.Store
	cfg    *config.Config
	client *http.Client
}

// NewPeerSyncClient creates a peer sync client.
func NewPeerSyncClient(st *store.Store, cfg *config.Config) *PeerSyncClient {
	return &PeerSyncClient{
		store:  st,
		cfg:    cfg,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// Run starts the sync loop.
func (c *PeerSyncClient) Run(ctx context.Context) {
	checkInterval := 10 * time.Second
	for {
		if !c.cfg.Network.Enabled || len(c.cfg.Network.Peers) == 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(checkInterval):
				continue
			}
		}

		interval := time.Duration(c.cfg.Network.SyncInterval) * time.Second
		if interval < 10*time.Second {
			interval = 10 * time.Second
		}
		ticker := time.NewTicker(interval)

		c.syncFromPeers(ctx)
		_ = c.store.PrunePeerData(c.cfg.Network.Peers)

	inner:
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				if !c.cfg.Network.Enabled || len(c.cfg.Network.Peers) == 0 {
					ticker.Stop()
					break inner
				}
				c.syncFromPeers(ctx)
				_ = c.store.PrunePeerData(c.cfg.Network.Peers)
			}
		}
	}
}

type filterResult struct {
	monitors []*monitor.Monitor
	warnings []string
}

func filterValidMonitors(monitors []*monitor.Monitor) filterResult {
	if len(monitors) == 0 {
		return filterResult{}
	}
	out := make([]*monitor.Monitor, 0, len(monitors))
	var warnings []string
	for _, m := range monitors {
		if m == nil {
			continue
		}
		if err := monitor.ValidateTarget(m.Type, m.Target); err != nil {
			msg := fmt.Sprintf("skipped invalid monitor %q: %v", m.Name, err)
			log.Printf("[sync] %s", msg)
			warnings = append(warnings, msg)
			continue
		}
		out = append(out, m)
	}
	return filterResult{monitors: out, warnings: warnings}
}

func (c *PeerSyncClient) syncFromPeers(ctx context.Context) {
	selfURL := strings.TrimSuffix(c.cfg.Network.SelfURL, "/")
	for _, peerURL := range c.cfg.Network.Peers {
		if peerURL == "" {
			continue
		}
		if selfURL != "" && strings.TrimSuffix(peerURL, "/") == selfURL {
			continue
		}
		url := strings.TrimSuffix(peerURL, "/") + "/api/sync/export"
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			url = "http://" + url
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			continue
		}
		setPeerSyncAuth(req, c.cfg)
		resp, err := c.client.Do(req)
		if err != nil {
			log.Printf("[sync] peer %s: %v", peerURL, err)
			_ = c.store.SetPeerDataError(peerURL, err.Error())
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			errMsg := fmt.Sprintf("HTTP %d", resp.StatusCode)
			if resp.StatusCode == http.StatusUnauthorized {
				if strings.TrimSpace(c.cfg.Network.SyncToken) != "" {
					errMsg = "HTTP 401 — check sync_token matches on both nodes"
				} else {
					errMsg = "HTTP 401 — set matching sync_token on all nodes or use identical web credentials"
				}
			}
			_ = c.store.SetPeerDataError(peerURL, errMsg)
			continue
		}
		var payload ExportPayload
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			resp.Body.Close()
			log.Printf("[sync] peer %s: decode error: %v", peerURL, err)
			_ = c.store.SetPeerDataError(peerURL, err.Error())
			continue
		}
		resp.Body.Close()

		if payload.NodeID == "" {
			continue
		}
		filtered := filterValidMonitors(payload.Monitors)
		exportTime := payload.Time
		if exportTime.IsZero() {
			exportTime = time.Now()
		}

		existing, _ := c.store.GetPeerData(payload.NodeID)
		data := &store.PeerData{
			NodeID:       payload.NodeID,
			PeerURL:      peerURL,
			LastSeen:     time.Now(),
			LastError:    "",
			SyncWarnings: filtered.warnings,
			LastExport:   exportTime,
		}
		if existing != nil && !exportTime.IsZero() && existing.LastExport.After(exportTime) {
			data.Monitors = existing.Monitors
			data.State = existing.State
			data.LastExport = existing.LastExport
		} else {
			data.Monitors = filtered.monitors
			incoming := payload.State
			if incoming == nil {
				incoming = make(map[string]*monitor.MonitorState)
			}
			if existing != nil {
				data.State = network.MergeStateMaps(existing.State, incoming)
			} else {
				data.State = incoming
			}
		}
		if data.State == nil {
			data.State = make(map[string]*monitor.MonitorState)
		}
		if err := c.store.SetPeerData(data); err != nil {
			log.Printf("[sync] save peer %s: %v", payload.NodeID, err)
		} else {
			log.Printf("[sync] peer %s: ok", peerURL)
		}
	}
}
