package commands

import (
	"context"

	"github.com/keshon/beacon/internal/service"
	"github.com/keshon/beacon/internal/store"
	"github.com/keshon/commandkit"
)

type StateGetCmd struct {
	store *store.Store
}

func (c *StateGetCmd) Name() string        { return "state:get" }
func (c *StateGetCmd) Description() string { return "Get all monitor state" }

func (c *StateGetCmd) Run(ctx context.Context, inv *commandkit.Invocation) error {
	d := getCLIData(inv)
	if d == nil {
		return nil
	}
	st, err := service.GetAllState(c.store)
	if err != nil {
		return err
	}
	writeJSONToWriter(d.Out, st)
	return nil
}

type EventsGetCmd struct {
	store *store.Store
}

func (c *EventsGetCmd) Name() string        { return "events:get" }
func (c *EventsGetCmd) Description() string { return "Get check history records" }

func (c *EventsGetCmd) Run(ctx context.Context, inv *commandkit.Invocation) error {
	d := getCLIData(inv)
	if d == nil {
		return nil
	}
	limit := service.ParseRecordLimit(d.Args["limit"])
	records, err := service.GetCheckRecords(c.store, limit)
	if err != nil {
		return err
	}
	writeJSONToWriter(d.Out, records)
	return nil
}
