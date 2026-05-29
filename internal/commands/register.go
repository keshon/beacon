package commands

import (
	"github.com/keshon/beacon/internal/store"

	"github.com/keshon/commandkit"
)

// RegisterAll registers CLI commands (monitor, state, events).
func RegisterAll(st *store.Store) {
	commandkit.DefaultRegistry.Register(&MonitorListCmd{store: st})
	commandkit.DefaultRegistry.Register(&MonitorAddCmd{store: st})
	commandkit.DefaultRegistry.Register(&MonitorDeleteCmd{store: st})
	commandkit.DefaultRegistry.Register(&MonitorUpdateCmd{store: st})
	commandkit.DefaultRegistry.Register(&StateGetCmd{store: st})
	commandkit.DefaultRegistry.Register(&EventsGetCmd{store: st})
}
