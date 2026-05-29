package service

import (
	"encoding/json"

	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/store"
)

func GetPublicConfig(st *store.Store) (config.PublicConfig, error) {
	var cfg config.Config
	ok, err := st.GetConfig(&cfg)
	if err != nil {
		return config.PublicConfig{}, err
	}
	if !ok {
		cfg = *config.Default()
	}
	cfg.Normalize()
	return cfg.ToPublic(), nil
}

func ApplyConfigPatch(st *store.Store, runtime *config.Config, body []byte) (config.PublicConfig, error) {
	var incoming config.Config
	if err := json.Unmarshal(body, &incoming); err != nil {
		return config.PublicConfig{}, ErrInvalidJSON
	}
	existing := *runtime
	ok, err := st.GetConfig(&existing)
	if err != nil {
		return config.PublicConfig{}, err
	}
	if !ok {
		existing = *config.Default()
	}
	config.ApplyNonSecret(&existing, &incoming)
	if err := config.MergeSecrets(&existing, &incoming); err != nil {
		return config.PublicConfig{}, err
	}
	existing.Normalize()
	if err := existing.Auth.EnsureHashed(); err != nil {
		return config.PublicConfig{}, err
	}
	if err := st.SetConfig(&existing); err != nil {
		return config.PublicConfig{}, err
	}
	*runtime = existing
	return existing.ToPublic(), nil
}
