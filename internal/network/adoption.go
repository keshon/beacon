package network

import (
	"sort"
	"time"

	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/store"
)

// AdoptedMonitor is a peer monitor checked locally because the owner node is dead.
type AdoptedMonitor struct {
	Monitor     *monitor.Monitor
	OwnerNodeID string
	OwnerLabel  string
}

// PeerLive reports whether peer data is within the dead timeout window.
func PeerLive(pd *store.PeerData, deadTimeout time.Duration, now time.Time) bool {
	if pd == nil || deadTimeout <= 0 {
		return false
	}
	return now.Sub(pd.LastSeen) < deadTimeout
}

// NextLiveInRing returns the next live node after deadID in sorted ring order.
func NextLiveInRing(sorted []string, deadID string, live map[string]bool) string {
	for i, id := range sorted {
		if id == deadID {
			for j := 1; j < len(sorted); j++ {
				idx := (i + j) % len(sorted)
				if live[sorted[idx]] {
					return sorted[idx]
				}
			}
			return ""
		}
	}
	return ""
}

// BuildRing collects node IDs and liveness from local config and peer cache.
func BuildRing(cfg *config.Config, peerData map[string]*store.PeerData, now time.Time, deadTimeout time.Duration) (sorted []string, live map[string]bool) {
	live = make(map[string]bool)
	if cfg == nil || !cfg.Network.Enabled || cfg.Network.NodeID == "" {
		return nil, live
	}
	sorted = append(sorted, cfg.Network.NodeID)
	live[cfg.Network.NodeID] = true
	for nodeID, pd := range peerData {
		sorted = append(sorted, nodeID)
		if PeerLive(pd, deadTimeout, now) {
			live[nodeID] = true
		}
	}
	sort.Strings(sorted)
	return sorted, live
}

// AdoptedMonitors returns enabled monitors this node should check for dead peers.
func AdoptedMonitors(cfg *config.Config, peerData map[string]*store.PeerData, now time.Time) []AdoptedMonitor {
	if cfg == nil || !cfg.Network.Enabled || cfg.Network.NodeID == "" || len(peerData) == 0 {
		return nil
	}
	deadTimeout := time.Duration(cfg.Network.DeadTimeout) * time.Second
	sorted, live := BuildRing(cfg, peerData, now, deadTimeout)
	var out []AdoptedMonitor
	seen := make(map[string]struct{})
	for _, pd := range peerData {
		if PeerLive(pd, deadTimeout, now) {
			continue
		}
		if NextLiveInRing(sorted, pd.NodeID, live) != cfg.Network.NodeID {
			continue
		}
		label := pd.PeerURL
		if label == "" {
			label = pd.NodeID
		}
		for _, m := range pd.Monitors {
			if m == nil || !m.Enabled {
				continue
			}
			if err := monitor.ValidateTarget(m.Type, m.Target); err != nil {
				continue
			}
			if _, ok := seen[m.ID]; ok {
				continue
			}
			seen[m.ID] = struct{}{}
			out = append(out, AdoptedMonitor{
				Monitor:     m,
				OwnerNodeID: pd.NodeID,
				OwnerLabel:  label,
			})
		}
	}
	return out
}

// MergeMonitorState picks the newer of two states by LastCheck.
func MergeMonitorState(a, b *monitor.MonitorState) *monitor.MonitorState {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if b.LastCheck.After(a.LastCheck) {
		return b
	}
	return a
}

// MergeStateMaps merges incoming into base, keeping newer LastCheck per monitor ID.
func MergeStateMaps(base, incoming map[string]*monitor.MonitorState) map[string]*monitor.MonitorState {
	if len(incoming) == 0 {
		return base
	}
	if base == nil {
		base = make(map[string]*monitor.MonitorState, len(incoming))
	}
	for id, st := range incoming {
		if st == nil {
			continue
		}
		base[id] = MergeMonitorState(base[id], st)
	}
	return base
}
