package runner

import (
	"strings"
	"time"

	"github.com/keshon/beacon/internal/monitor/checks"
	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/storage"
)

// UpdatePatch is a partial monitor update (nil fields are left unchanged).
type UpdatePatch struct {
	Enabled        *bool
	Name           *string
	Type           *string
	Target         *string
	IntervalSec    *int
	HTTP           *checks.HTTPOptions
	NotifyOverride *monitor.NotifyOverride
}

// Update applies patch to an existing monitor and returns the updated record.
func Update(st *storage.Store, id string, patch UpdatePatch) (*monitor.Monitor, error) {
	return st.UpdateMonitor(id, func(mon *monitor.Monitor) error {
		if patch.Enabled != nil {
			mon.Enabled = *patch.Enabled
		}
		if patch.Name != nil {
			mon.Name = *patch.Name
		}
		if patch.Type != nil {
			typ, err := monitor.NormalizeType(*patch.Type)
			if err != nil {
				return err
			}
			mon.Type = typ
		}
		if patch.Target != nil {
			mon.Target = strings.TrimSpace(*patch.Target)
		}
		if patch.Type != nil || patch.Target != nil {
			if err := monitor.ValidateTarget(mon.Type, mon.Target); err != nil {
				return err
			}
		}
		if patch.IntervalSec != nil {
			if *patch.IntervalSec > 0 {
				mon.Interval = time.Duration(*patch.IntervalSec) * time.Second
			} else {
				mon.Interval = 0
			}
		}
		if patch.HTTP != nil {
			mon.HTTP = monitor.MergeHTTPOptions(mon.HTTP, patch.HTTP)
		}
		if patch.NotifyOverride != nil {
			sanitized := monitor.SanitizeNotifyOverride(patch.NotifyOverride)
			if sanitized != nil {
				mon.NotifyOverride = sanitized
			}
		}
		return nil
	})
}
