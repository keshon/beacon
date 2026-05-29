package commands

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/store"
	"github.com/keshon/commandkit"
)

type ConfigGetCmd struct {
	store *store.Store
}

func (c *ConfigGetCmd) Name() string        { return "config:get" }
func (c *ConfigGetCmd) Description() string { return "Get config" }

func (c *ConfigGetCmd) Run(ctx context.Context, inv *commandkit.Invocation) error {
	if d := getHTTPData(inv); d != nil {
		var cfg config.Config
		ok, err := c.store.GetConfig(&cfg)
		if err != nil {
			http.Error(d.W, err.Error(), http.StatusInternalServerError)
			return nil
		}
		if !ok {
			cfg = *config.Default()
		}
		cfg.Normalize()
		writeJSONTo(d.W, cfg.ToPublic())
		return nil
	}
	return nil
}

type ConfigSetCmd struct {
	store *store.Store
	cfg   *config.Config
}

func (c *ConfigSetCmd) Name() string        { return "config:set" }
func (c *ConfigSetCmd) Description() string { return "Save config" }

func (c *ConfigSetCmd) Run(ctx context.Context, inv *commandkit.Invocation) error {
	d := getHTTPData(inv)
	if d == nil {
		return nil
	}
	var incoming config.Config
	if err := json.NewDecoder(d.R.Body).Decode(&incoming); err != nil {
		http.Error(d.W, "invalid JSON", http.StatusBadRequest)
		return nil
	}
	var existing config.Config
	ok, err := c.store.GetConfig(&existing)
	if err != nil {
		http.Error(d.W, err.Error(), http.StatusInternalServerError)
		return nil
	}
	if !ok {
		existing = *config.Default()
	}
	config.ApplyNonSecret(&existing, &incoming)
	if err := config.MergeSecrets(&existing, &incoming); err != nil {
		http.Error(d.W, err.Error(), http.StatusBadRequest)
		return nil
	}
	existing.Normalize()
	if err := existing.Auth.EnsureAuthHashed(); err != nil {
		http.Error(d.W, err.Error(), http.StatusInternalServerError)
		return nil
	}
	if err := c.store.SetConfig(&existing); err != nil {
		http.Error(d.W, err.Error(), http.StatusInternalServerError)
		return nil
	}
	*c.cfg = existing
	writeJSONTo(d.W, existing.ToPublic())
	return nil
}
