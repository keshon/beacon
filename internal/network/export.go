package network

import (
	"time"

	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/store"
)

// ExportView is the monitor/state pair served to peers and the dashboard.
type ExportView struct {
	Monitors []*monitor.Monitor
	State    map[string]*monitor.MonitorState
}

// BuildExportView returns local monitors plus adopted peer monitors for downstream sync.
func BuildExportView(cfg *config.Config, st *store.Store) (ExportView, error) {
	var view ExportView
	snap, err := st.GetExportSnapshot()
	if err != nil {
		return view, err
	}
	view.Monitors = append(view.Monitors, snap.Monitors...)
	view.State = snap.State
	if view.State == nil {
		view.State = make(map[string]*monitor.MonitorState)
	}

	if cfg == nil || !cfg.Network.Enabled || cfg.Network.NodeID == "" {
		return view, nil
	}

	peerData, err := st.GetAllPeerData()
	if err != nil {
		return view, err
	}
	seen := make(map[string]struct{}, len(view.Monitors))
	for _, m := range view.Monitors {
		if m != nil {
			seen[m.ID] = struct{}{}
		}
	}
	now := time.Now()
	for _, am := range AdoptedMonitors(cfg, peerData, now) {
		if am.Monitor == nil {
			continue
		}
		if _, ok := seen[am.Monitor.ID]; ok {
			continue
		}
		seen[am.Monitor.ID] = struct{}{}
		copy := *am.Monitor
		if copy.OwnerNodeID == "" {
			copy.OwnerNodeID = am.OwnerNodeID
		}
		view.Monitors = append(view.Monitors, &copy)
		if pd := peerData[am.OwnerNodeID]; pd != nil {
			if pst, ok := pd.State[am.Monitor.ID]; ok && pst != nil {
				local := view.State[am.Monitor.ID]
				view.State[am.Monitor.ID] = MergeMonitorState(local, pst)
			}
		}
	}
	return view, nil
}
