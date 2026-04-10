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

// Client periodically fetches data from peer nodes.
type Client struct {
	store  *store.Store
	cfg    *config.Config
	client *http.Client
}

// NewClient creates a sync client.
func NewClient(st *store.Store, cfg *config.Config) *Client {
	return &Client{
		store:  st,
		cfg:    cfg,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// Run starts the sync loop. It fetches from each peer and stores the result.
// The loop keeps running even when network is disabled, re-checking config periodically
// so that enabling network in Settings takes effect without restart.
func (c *Client) Run(ctx context.Context) {
	checkInterval := 10 * time.Second
	for {
		// Re-check config: if disabled or no peers, sleep and re-check
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

		// Immediate first sync when network becomes enabled
		c.syncFromPeers()

		// Ticker loop
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
				c.syncFromPeers()
			}
		}
	}
}

func (c *Client) syncFromPeers() {
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
		req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
		if err != nil {
			continue
		}
		req.SetBasicAuth(c.cfg.Auth.Username, c.cfg.Auth.Password)
		resp, err := c.client.Do(req)
		if err != nil {
			log.Printf("[sync] peer %s: %v", peerURL, err)
			c.recordSyncError(peerURL, err.Error())
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			c.recordSyncError(peerURL, fmt.Sprintf("HTTP %d", resp.StatusCode))
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
			Monitors:  payload.Monitors,
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

func (c *Client) recordSyncError(peerURL, errMsg string) {
	all := c.store.GetAllPeerData()
	normalized := strings.TrimSuffix(peerURL, "/")
	for _, pd := range all {
		if strings.TrimSuffix(pd.PeerURL, "/") == normalized {
			pd.LastError = errMsg
			c.store.SetPeerData(pd)
			return
		}
	}
}
