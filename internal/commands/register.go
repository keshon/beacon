package commands

import (
	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/store"

	"github.com/keshon/commandkit"
)

// RegisterAll registers monitor, state, and event commands.
func RegisterAll(st *store.Store) {
	commandkit.DefaultRegistry.Register(&MonitorListCmd{store: st})
	commandkit.DefaultRegistry.Register(&MonitorAddCmd{store: st})
	commandkit.DefaultRegistry.Register(&MonitorDeleteCmd{store: st})
	commandkit.DefaultRegistry.Register(&MonitorUpdateCmd{store: st})
	commandkit.DefaultRegistry.Register(&StateGetCmd{store: st})
	commandkit.DefaultRegistry.Register(&EventsGetCmd{store: st})
}

// RegisterConfigCommands registers config commands. cfg is updated in-place when config is saved.
func RegisterConfigCommands(st *store.Store, cfg *config.Config) {
	commandkit.DefaultRegistry.Register(&ConfigGetCmd{store: st})
	commandkit.DefaultRegistry.Register(&ConfigSetCmd{store: st, cfg: cfg})
}
