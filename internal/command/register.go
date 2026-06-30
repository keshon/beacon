package command

import (
	"fmt"

	cmdlib "github.com/keshon/command"
	"github.com/keshon/beacon/internal/store"
)

// RegisterAll registers CLI commands on the default registry.
func RegisterAll(st *store.Store, wrap func(cmdlib.Command) cmdlib.Command) {
	register := func(c cmdlib.Command) {
		if wrap != nil {
			c = wrap(c)
		}
		cmdlib.DefaultRegistry.Register(c)
	}
	register(&MonitorList{Store: st})
	register(&MonitorAdd{Store: st})
	register(&MonitorDelete{Store: st})
	register(&MonitorUpdate{Store: st})
	register(&StateGet{Store: st})
	register(&EventsGet{Store: st})
}

// Resolve maps beacon monitor/state/events argv to a registry command name and args.
func Resolve(argv []string) (name string, args []string, ok bool) {
	if len(argv) < 2 {
		return "", nil, false
	}
	switch argv[1] {
	case "monitor":
		if len(argv) < 3 {
			return "", nil, true
		}
		return "monitor:" + argv[2], argv[3:], true
	case "state":
		return "state:get", argv[2:], true
	case "events":
		return "events:get", argv[2:], true
	default:
		return "", nil, false
	}
}

// IsCLISubcommand reports whether argv starts a CLI invocation.
func IsCLISubcommand(argv []string) bool {
	if len(argv) < 2 {
		return false
	}
	switch argv[1] {
	case "monitor", "state", "events":
		return true
	default:
		return false
	}
}

// UsageHint returns a short CLI usage line.
func UsageHint(sub string) string {
	if sub == "monitor" {
		return "Usage: beacon monitor <list|add|delete|update> [args]"
	}
	return ""
}

// ErrMonitorUsage is returned when monitor argv is incomplete.
var ErrMonitorUsage = fmt.Errorf("usage: beacon monitor <list|add|delete|update> [args]")
