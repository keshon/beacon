package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/store"
)

func (rt *Runtime) syncFromPeers(ctx context.Context) {
	selfURL := strings.TrimSuffix(rt.cfg.Network.SelfURL, "/")
	for _, peerURL := range rt.cfg.Network.Peers {
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
		setOutboundSyncAuth(req, rt.cfg)
		resp, err := rt.client.Do(req)
		if err != nil {
			log.Printf("[cluster] peer %s: %v", peerURL, err)
			_ = rt.store.SetPeerDataError(peerURL, err.Error())
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			errMsg := fmt.Sprintf("HTTP %d", resp.StatusCode)
			if resp.StatusCode == http.StatusUnauthorized {
				if strings.TrimSpace(rt.cfg.Network.SyncToken) != "" {
					errMsg = "HTTP 401 — check sync_token matches on both nodes"
				} else {
					errMsg = "HTTP 401 — set matching sync_token on all nodes or use identical web credentials"
				}
			}
			_ = rt.store.SetPeerDataError(peerURL, errMsg)
			continue
		}
		var payload ExportPayload
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			resp.Body.Close()
			log.Printf("[cluster] peer %s: decode error: %v", peerURL, err)
			_ = rt.store.SetPeerDataError(peerURL, err.Error())
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

		existing, _ := rt.store.GetPeerData(payload.NodeID)
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
				data.State = mergeStateMaps(existing.State, incoming)
			} else {
				data.State = incoming
			}
		}
		if data.State == nil {
			data.State = make(map[string]*monitor.MonitorState)
		}
		if err := rt.store.SetPeerData(data); err != nil {
			log.Printf("[cluster] save peer %s: %v", payload.NodeID, err)
		} else {
			log.Printf("[cluster] peer %s: ok", peerURL)
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
			log.Printf("[cluster] %s", msg)
			warnings = append(warnings, msg)
			continue
		}
		out = append(out, m)
	}
	return filterResult{monitors: out, warnings: warnings}
}

func (rt *Runtime) runSync(ctx context.Context) {
	checkInterval := 10 * time.Second
	for {
		if !rt.cfg.Network.Enabled || len(rt.cfg.Network.Peers) == 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(checkInterval):
				continue
			}
		}

		interval := time.Duration(rt.cfg.Network.SyncInterval) * time.Second
		if interval < 10*time.Second {
			interval = 10 * time.Second
		}
		ticker := time.NewTicker(interval)

		rt.syncFromPeers(ctx)
		_ = rt.store.PrunePeerData(rt.cfg.Network.Peers)

	inner:
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				if !rt.cfg.Network.Enabled || len(rt.cfg.Network.Peers) == 0 {
					ticker.Stop()
					break inner
				}
				rt.syncFromPeers(ctx)
				_ = rt.store.PrunePeerData(rt.cfg.Network.Peers)
			}
		}
	}
}
