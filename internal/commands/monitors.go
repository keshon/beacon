package commands

import (
	"context"
	"fmt"

	"github.com/keshon/beacon/internal/service"
	"github.com/keshon/beacon/internal/store"
	"github.com/keshon/commandkit"
)

type MonitorListCmd struct {
	store *store.Store
}

func (c *MonitorListCmd) Name() string        { return "monitor:list" }
func (c *MonitorListCmd) Description() string { return "List all monitors" }

func (c *MonitorListCmd) Run(ctx context.Context, inv *commandkit.Invocation) error {
	d := getCLIData(inv)
	if d == nil {
		return nil
	}
	list, err := service.ListMonitors(c.store)
	if err != nil {
		return err
	}
	writeJSONToWriter(d.Out, list)
	return nil
}

type MonitorAddCmd struct {
	store *store.Store
}

func (c *MonitorAddCmd) Name() string        { return "monitor:add" }
func (c *MonitorAddCmd) Description() string { return "Add a monitor" }

func (c *MonitorAddCmd) Run(ctx context.Context, inv *commandkit.Invocation) error {
	d := getCLIData(inv)
	if d == nil {
		return nil
	}
	m, err := service.AddMonitor(c.store, service.AddMonitorInput{
		Name:   d.Args["name"],
		Type:   d.Args["type"],
		Target: d.Args["target"],
	})
	if err != nil {
		return err
	}
	writeJSONToWriter(d.Out, m)
	return nil
}

type MonitorDeleteCmd struct {
	store *store.Store
}

func (c *MonitorDeleteCmd) Name() string        { return "monitor:delete" }
func (c *MonitorDeleteCmd) Description() string { return "Delete a monitor" }

func (c *MonitorDeleteCmd) Run(ctx context.Context, inv *commandkit.Invocation) error {
	d := getCLIData(inv)
	if d == nil {
		return nil
	}
	id := d.Args["id"]
	if id == "" {
		return nil
	}
	return service.DeleteMonitor(c.store, id)
}

type MonitorUpdateCmd struct {
	store *store.Store
}

func (c *MonitorUpdateCmd) Name() string        { return "monitor:update" }
func (c *MonitorUpdateCmd) Description() string { return "Update a monitor" }

func (c *MonitorUpdateCmd) Run(ctx context.Context, inv *commandkit.Invocation) error {
	d := getCLIData(inv)
	if d == nil {
		return nil
	}
	id := d.Args["id"]
	if id == "" {
		return nil
	}
	var enabled *bool
	switch d.Args["enabled"] {
	case "true":
		b := true
		enabled = &b
	case "false":
		b := false
		enabled = &b
	}
	m, err := service.UpdateMonitor(c.store, id, service.UpdateMonitorPatch{Enabled: enabled})
	if err != nil {
		return err
	}
	if m == nil {
		return fmt.Errorf("monitor not found")
	}
	writeJSONToWriter(d.Out, m)
	return nil
}
