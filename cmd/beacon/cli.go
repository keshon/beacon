package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/store"
)

func isCLISubcommand(s string) bool {
	return s == "monitor" || s == "state" || s == "events"
}

func runCLI(st *store.Store) bool {
	if len(os.Args) < 2 {
		return false
	}
	sub := os.Args[1]
	if !isCLISubcommand(sub) {
		return false
	}

	lock, err := acquireDataDirLock("data")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return true
	}
	defer releaseDataDirLock(lock)

	switch sub {
	case "monitor":
		return runMonitorCLI(st)
	case "state":
		return runStateCLI(st)
	case "events":
		return runEventsCLI(st)
	}
	return false
}

func runMonitorCLI(st *store.Store) bool {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: beacon monitor <list|add|delete|update> [args]")
		return true
	}
	action := os.Args[2]
	args := os.Args[3:]

	switch action {
	case "list":
		list, err := st.GetMonitors()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return true
		}
		writeCLIJSON(os.Stdout, list)
	case "add":
		fs := flag.NewFlagSet("add", flag.ExitOnError)
		name := fs.String("name", "", "Monitor name")
		typ := fs.String("type", "http", "Monitor type (http|tcp)")
		target := fs.String("target", "", "Target URL or host:port")
		fs.Parse(args)
		if *name == "" || *target == "" {
			fmt.Fprintln(os.Stderr, "name and target are required")
			return true
		}
		m, err := cliAddMonitor(st, *name, *typ, *target)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return true
		}
		writeCLIJSON(os.Stdout, m)
	case "delete":
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "Usage: beacon monitor delete <id>")
			return true
		}
		if err := st.DeleteMonitor(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	case "update", "enable", "disable":
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "Usage: beacon monitor update <id> [--enable|--disable]")
			return true
		}
		fs := flag.NewFlagSet("update", flag.ExitOnError)
		enable := fs.Bool("enable", false, "Enable monitor")
		disable := fs.Bool("disable", false, "Disable monitor")
		fs.Parse(args[1:])
		id := args[0]
		m, err := st.UpdateMonitor(id, func(m *monitor.Monitor) error {
			if *enable {
				m.Enabled = true
			}
			if *disable {
				m.Enabled = false
			}
			return nil
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return true
		}
		writeCLIJSON(os.Stdout, m)
	default:
		fmt.Fprintf(os.Stderr, "Unknown action: %s\n", action)
	}
	return true
}

func cliAddMonitor(st *store.Store, name, typ, target string) (*monitor.Monitor, error) {
	t, err := monitor.NormalizeType(typ)
	if err != nil {
		return nil, err
	}
	target = strings.TrimSpace(target)
	if err := monitor.ValidateTarget(t, target); err != nil {
		return nil, err
	}
	m := &monitor.Monitor{
		ID:       uuid.New().String(),
		Name:     strings.TrimSpace(name),
		Type:     t,
		Target:   target,
		Timeout:  10 * time.Second,
		Retries:  3,
		Enabled:  true,
	}
	if err := st.SetMonitor(m); err != nil {
		return nil, err
	}
	return m, nil
}

func runStateCLI(st *store.Store) bool {
	states, err := st.GetAllState()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return true
	}
	writeCLIJSON(os.Stdout, states)
	return true
}

func runEventsCLI(st *store.Store) bool {
	limit := 100
	if len(os.Args) >= 3 {
		fs := flag.NewFlagSet("events", flag.ExitOnError)
		n := fs.Int("limit", 100, "Max events to return")
		fs.Parse(os.Args[2:])
		limit = *n
	}
	records, err := st.GetCheckRecords(limit)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return true
	}
	writeCLIJSON(os.Stdout, records)
	return true
}

func writeCLIJSON(w io.Writer, v any) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
