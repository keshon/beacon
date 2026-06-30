package main

import (
	"context"
	"fmt"
	"os"

	cmdlib "github.com/keshon/command"
	beaconcmd "github.com/keshon/beacon/internal/command"
	"github.com/keshon/beacon/internal/store"
)

func withDataDirLock(dataDir string, next cmdlib.Command) cmdlib.Command {
	return cmdlib.Wrap(next, func(ctx context.Context, inv *cmdlib.Invocation) error {
		lock, err := acquireDataDirLock(dataDir)
		if err != nil {
			return err
		}
		defer releaseDataDirLock(lock)
		return next.Run(ctx, inv)
	})
}

func runCLI(st *store.Store, dataDir string) bool {
	if !beaconcmd.IsCLISubcommand(os.Args) {
		return false
	}

	beaconcmd.RegisterAll(st, func(c cmdlib.Command) cmdlib.Command {
		return withDataDirLock(dataDir, c)
	})

	name, args, ok := beaconcmd.Resolve(os.Args)
	if !ok {
		return false
	}
	if name == "" {
		fmt.Fprintln(os.Stderr, beaconcmd.UsageHint("monitor"))
		return true
	}

	cmd := cmdlib.DefaultRegistry.Get(name)
	if cmd == nil {
		fmt.Fprintf(os.Stderr, "Unknown action: %s\n", name)
		return true
	}

	cliCtx := &beaconcmd.CLIContext{Store: st, Out: os.Stdout}
	if err := cmd.Run(context.Background(), &cmdlib.Invocation{Args: args, Data: cliCtx}); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return true
}
