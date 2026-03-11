package cli

import (
	"context"
	"flag"
	"fmt"
	"os"

	"beacon/internal/commands"
	"beacon/internal/store"

	"github.com/keshon/commandkit"
)

// RunCLI runs the CLI and returns true if a CLI command was executed.
// When false, the caller should start the server.
func RunCLI(st *store.Store) bool {
	if len(os.Args) < 2 {
		return false
	}
	sub := os.Args[1]
	if sub != "monitor" && sub != "state" && sub != "events" {
		return false
	}

	commands.RegisterAll(st)
	ctx := context.Background()

	switch sub {
	case "monitor":
		return runMonitor(ctx, st)
	case "state":
		return runState(ctx, st)
	case "events":
		return runEvents(ctx, st)
	}
	return false
}

func runMonitor(ctx context.Context, st *store.Store) bool {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: Beacon <list|add|delete|update> [args]")
		return true
	}
	action := os.Args[2]
	args := os.Args[3:]

	switch action {
	case "list":
		cmd := commandkit.DefaultRegistry.Get("monitor:list")
		if cmd == nil {
			return true
		}
		inv := &commandkit.Invocation{Data: &commands.CLIData{Out: os.Stdout}}
		cmd.Run(ctx, inv)
		return true
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
		cmd := commandkit.DefaultRegistry.Get("monitor:add")
		if cmd == nil {
			return true
		}
		inv := &commandkit.Invocation{
			Data: &commands.CLIData{
				Out:  os.Stdout,
				Args: map[string]string{"name": *name, "type": *typ, "target": *target},
			},
		}
		cmd.Run(ctx, inv)
		return true
	case "delete":
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "Usage: Beacon delete <id>")
			return true
		}
		cmd := commandkit.DefaultRegistry.Get("monitor:delete")
		if cmd == nil {
			return true
		}
		inv := &commandkit.Invocation{
			Data: &commands.CLIData{Out: os.Stdout, Args: map[string]string{"id": args[0]}},
		}
		cmd.Run(ctx, inv)
		return true
	case "update", "enable", "disable":
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "Usage: Beacon update <id> [--enable|--disable]")
			return true
		}
		fs := flag.NewFlagSet("update", flag.ExitOnError)
		enable := fs.Bool("enable", false, "Enable monitor")
		disable := fs.Bool("disable", false, "Disable monitor")
		fs.Parse(args[1:])
		enabled := ""
		if *enable {
			enabled = "true"
		} else if *disable {
			enabled = "false"
		}
		cmd := commandkit.DefaultRegistry.Get("monitor:update")
		if cmd == nil {
			return true
		}
		inv := &commandkit.Invocation{
			Data: &commands.CLIData{
				Out:  os.Stdout,
				Args: map[string]string{"id": args[0], "enabled": enabled},
			},
		}
		cmd.Run(ctx, inv)
		return true
	default:
		fmt.Fprintf(os.Stderr, "Unknown action: %s\n", action)
		return true
	}
}

func runState(ctx context.Context, st *store.Store) bool {
	cmd := commandkit.DefaultRegistry.Get("state:get")
	if cmd == nil {
		return true
	}
	inv := &commandkit.Invocation{Data: &commands.CLIData{Out: os.Stdout}}
	cmd.Run(ctx, inv)
	return true
}

func runEvents(ctx context.Context, st *store.Store) bool {
	if len(os.Args) < 3 {
		cmd := commandkit.DefaultRegistry.Get("events:get")
		if cmd == nil {
			return true
		}
		inv := &commandkit.Invocation{Data: &commands.CLIData{Out: os.Stdout}}
		cmd.Run(ctx, inv)
		return true
	}
	fs := flag.NewFlagSet("events", flag.ExitOnError)
	limit := fs.Int("limit", 100, "Max events to return")
	fs.Parse(os.Args[2:])
	cmd := commandkit.DefaultRegistry.Get("events:get")
	if cmd == nil {
		return true
	}
	inv := &commandkit.Invocation{
		Data: &commands.CLIData{
			Out:  os.Stdout,
			Args: map[string]string{"limit": fmt.Sprint(*limit)},
		},
	}
	cmd.Run(ctx, inv)
	return true
}
