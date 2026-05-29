package commands

import (
	"context"
	"net/http"
	"strconv"

	"github.com/keshon/beacon/internal/store"
	"github.com/keshon/commandkit"
)

type StateGetCmd struct {
	store *store.Store
}

func (c *StateGetCmd) Name() string        { return "state:get" }
func (c *StateGetCmd) Description() string { return "Get all monitor state" }

func (c *StateGetCmd) Run(ctx context.Context, inv *commandkit.Invocation) error {
	if d := getHTTPData(inv); d != nil {
		st, err := c.store.GetAllState()
		if err != nil {
			http.Error(d.W, err.Error(), http.StatusInternalServerError)
			return nil
		}
		writeJSONTo(d.W, st)
		return nil
	}
	if d := getCLIData(inv); d != nil {
		st, err := c.store.GetAllState()
		if err != nil {
			return err
		}
		writeJSONToWriter(d.Out, st)
		return nil
	}
	return nil
}

type EventsGetCmd struct {
	store *store.Store
}

func (c *EventsGetCmd) Name() string        { return "events:get" }
func (c *EventsGetCmd) Description() string { return "Get check history records" }

func (c *EventsGetCmd) Run(ctx context.Context, inv *commandkit.Invocation) error {
	limit := 100
	if d := getHTTPData(inv); d != nil {
		if l := d.R.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 {
				limit = n
			}
		}
		records, err := c.store.GetCheckRecords(limit)
		if err != nil {
			http.Error(d.W, err.Error(), http.StatusInternalServerError)
			return nil
		}
		writeJSONTo(d.W, records)
		return nil
	}
	if d := getCLIData(inv); d != nil {
		if l := d.Args["limit"]; l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 {
				limit = n
			}
		}
		records, err := c.store.GetCheckRecords(limit)
		if err != nil {
			return err
		}
		writeJSONToWriter(d.Out, records)
		return nil
	}
	return nil
}
