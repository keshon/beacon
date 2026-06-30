package command

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	cmdlib "github.com/keshon/command"
	"github.com/keshon/beacon/internal/monitorsvc"
	"github.com/keshon/beacon/internal/store"
)

func ctxData(inv *cmdlib.Invocation) *CLIContext {
	if inv == nil || inv.Data == nil {
		return &CLIContext{Out: os.Stdout}
	}
	d, ok := inv.Data.(*CLIContext)
	if !ok {
		return &CLIContext{Out: os.Stdout}
	}
	if d.Out == nil {
		d.Out = os.Stdout
	}
	return d
}

func writeJSON(w io.Writer, v any) error {
	if w == nil {
		w = os.Stdout
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}

type MonitorList struct {
	Store *store.Store
}

func (c *MonitorList) Name() string        { return "monitor:list" }
func (c *MonitorList) Description() string { return "List all monitors" }

func (c *MonitorList) Run(ctx context.Context, inv *cmdlib.Invocation) error {
	_ = ctx
	cli := ctxData(inv)
	list, err := monitorsvc.List(c.Store)
	if err != nil {
		return err
	}
	return writeJSON(cli.Out, list)
}

type MonitorAdd struct {
	Store *store.Store
}

func (c *MonitorAdd) Name() string        { return "monitor:add" }
func (c *MonitorAdd) Description() string { return "Add a monitor" }

func (c *MonitorAdd) Run(ctx context.Context, inv *cmdlib.Invocation) error {
	_ = ctx
	cli := ctxData(inv)
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(cli.Out)
	name := fs.String("name", "", "Monitor name")
	typ := fs.String("type", "http", "Monitor type (http|tcp)")
	target := fs.String("target", "", "Target URL or host:port")
	if err := fs.Parse(inv.Args); err != nil {
		return err
	}
	m, err := monitorsvc.Add(c.Store, monitorsvc.AddInput{
		Name:   *name,
		Type:   *typ,
		Target: *target,
	})
	if err != nil {
		return err
	}
	return writeJSON(cli.Out, m)
}

type MonitorDelete struct {
	Store *store.Store
}

func (c *MonitorDelete) Name() string        { return "monitor:delete" }
func (c *MonitorDelete) Description() string { return "Delete a monitor" }

func (c *MonitorDelete) Run(ctx context.Context, inv *cmdlib.Invocation) error {
	_ = ctx
	if len(inv.Args) < 1 {
		return fmt.Errorf("usage: beacon monitor delete <id>")
	}
	return monitorsvc.Delete(c.Store, inv.Args[0])
}

type MonitorUpdate struct {
	Store *store.Store
}

func (c *MonitorUpdate) Name() string        { return "monitor:update" }
func (c *MonitorUpdate) Description() string { return "Update a monitor" }

func (c *MonitorUpdate) Run(ctx context.Context, inv *cmdlib.Invocation) error {
	_ = ctx
	cli := ctxData(inv)
	if len(inv.Args) < 1 {
		return fmt.Errorf("usage: beacon monitor update <id> [--enable|--disable]")
	}
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(cli.Out)
	enable := fs.Bool("enable", false, "Enable monitor")
	disable := fs.Bool("disable", false, "Disable monitor")
	if err := fs.Parse(inv.Args[1:]); err != nil {
		return err
	}
	patch := monitorsvc.UpdatePatch{}
	if *enable {
		v := true
		patch.Enabled = &v
	}
	if *disable {
		v := false
		patch.Enabled = &v
	}
	m, err := monitorsvc.Update(c.Store, inv.Args[0], patch)
	if err != nil {
		return err
	}
	return writeJSON(cli.Out, m)
}
