package cluster

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

func peerLive(pd *store.PeerData, deadTimeout time.Duration, now time.Time) bool {
	if pd == nil || deadTimeout <= 0 {
		return false
	}
	return now.Sub(pd.LastSeen) < deadTimeout
}

func nextLiveInRing(sorted []string, deadID string, live map[string]bool) string {
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

func buildRing(cfg *config.Config, peerData map[string]*store.PeerData, now time.Time, deadTimeout time.Duration) (sorted []string, live map[string]bool) {
	live = make(map[string]bool)
	if cfg == nil || !cfg.Network.Enabled || cfg.Network.NodeID == "" {
		return nil, live
	}
	sorted = append(sorted, cfg.Network.NodeID)
	live[cfg.Network.NodeID] = true
	for nodeID, pd := range peerData {
		sorted = append(sorted, nodeID)
		if peerLive(pd, deadTimeout, now) {
			live[nodeID] = true
		}
	}
	sort.Strings(sorted)
	return sorted, live
}

func adoptedMonitors(cfg *config.Config, peerData map[string]*store.PeerData, now time.Time) []AdoptedMonitor {
	if cfg == nil || !cfg.Network.Enabled || cfg.Network.NodeID == "" || len(peerData) == 0 {
		return nil
	}
	deadTimeout := time.Duration(cfg.Network.DeadTimeout) * time.Second
	sorted, live := buildRing(cfg, peerData, now, deadTimeout)
	var out []AdoptedMonitor
	seen := make(map[string]struct{})
	for _, pd := range peerData {
		if peerLive(pd, deadTimeout, now) {
			continue
		}
		if nextLiveInRing(sorted, pd.NodeID, live) != cfg.Network.NodeID {
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
	return mergeMonitorState(a, b)
}

// MergeStateMaps merges incoming state into base, keeping newer LastCheck per ID.
func MergeStateMaps(base, incoming map[string]*monitor.MonitorState) map[string]*monitor.MonitorState {
	return mergeStateMaps(base, incoming)
}

func mergeMonitorState(a, b *monitor.MonitorState) *monitor.MonitorState {
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

func mergeStateMaps(base, incoming map[string]*monitor.MonitorState) map[string]*monitor.MonitorState {
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
		base[id] = mergeMonitorState(base[id], st)
	}
	return base
}
