package config

import "sync/atomic"

// Live is the shared runtime config holder. Readers call Load and get an
// immutable snapshot; writers build a new Config and Store it. A loaded
// *Config must never be mutated — copy, modify, Store.
type Live struct {
	p atomic.Pointer[Config]
}

// NewLive wraps cfg in a Live holder. A nil cfg falls back to Default().
func NewLive(cfg *Config) *Live {
	if cfg == nil {
		cfg = Default()
	}
	l := &Live{}
	l.p.Store(cfg)
	return l
}

// Load returns the current config snapshot. Never nil.
func (l *Live) Load() *Config {
	return l.p.Load()
}

// Store publishes a new config snapshot. Nil is ignored.
func (l *Live) Store(cfg *Config) {
	if cfg == nil {
		return
	}
	l.p.Store(cfg)
}
