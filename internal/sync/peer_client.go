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
			}
		}
	}
}

func filterValidMonitors(monitors []*monitor.Monitor) []*monitor.Monitor {
	if len(monitors) == 0 {
		return nil
	}
	out := make([]*monitor.Monitor, 0, len(monitors))
	for _, m := range monitors {
		if m == nil {
			continue
		}
		if err := monitor.ValidateTarget(m.Type, m.Target); err != nil {
			log.Printf("[sync] skip invalid monitor %s: %v", m.Name, err)
			continue
		}
		out = append(out, m)
	}
	return out
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
			c.recordSyncError(peerURL, err.Error())
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
			c.recordSyncError(peerURL, errMsg)
			continue
		}
		var payload ExportPayload
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			resp.Body.Close()
			log.Printf("[sync] peer %s: decode error: %v", peerURL, err)
			c.recordSyncError(peerURL, err.Error())
			continue
		}
		resp.Body.Close()

		if payload.NodeID == "" {
			continue
		}
		data := &store.PeerData{
			NodeID:    payload.NodeID,
			PeerURL:   peerURL,
			Monitors:  filterValidMonitors(payload.Monitors),
			State:     payload.State,
			LastSeen:  time.Now(),
			LastError: "",
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

func (c *PeerSyncClient) recordSyncError(peerURL, errMsg string) {
	all, err := c.store.GetAllPeerData()
	if err != nil {
		return
	}
	normalized := strings.TrimSuffix(peerURL, "/")
	for _, pd := range all {
		if strings.TrimSuffix(pd.PeerURL, "/") == normalized {
			pd.LastError = errMsg
			_ = c.store.SetPeerData(pd)
			return
		}
	}
}
