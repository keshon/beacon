package command

import (
	"context"
	"flag"

	cmdlib "github.com/keshon/command"
	"github.com/keshon/beacon/internal/store"
)

type StateGet struct {
	Store *store.Store
}

func (c *StateGet) Name() string        { return "state:get" }
func (c *StateGet) Description() string { return "Get all monitor state" }

func (c *StateGet) Run(ctx context.Context, inv *cmdlib.Invocation) error {
	_ = ctx
	cli := ctxData(inv)
	states, err := c.Store.GetAllState()
	if err != nil {
		return err
	}
	return writeJSON(cli.Out, states)
}

type EventsGet struct {
	Store *store.Store
}

func (c *EventsGet) Name() string        { return "events:get" }
func (c *EventsGet) Description() string { return "Get check history records" }

func (c *EventsGet) Run(ctx context.Context, inv *cmdlib.Invocation) error {
	_ = ctx
	cli := ctxData(inv)
	limit := 100
	if len(inv.Args) > 0 {
		fs := flag.NewFlagSet("events", flag.ContinueOnError)
		fs.SetOutput(cli.Out)
		n := fs.Int("limit", 100, "Max events to return")
		if err := fs.Parse(inv.Args); err != nil {
			return err
		}
		limit = *n
	}
	records, err := c.Store.GetCheckRecords(limit)
	if err != nil {
		return err
	}
	return writeJSON(cli.Out, records)
}
