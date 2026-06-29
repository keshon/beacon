package scheduler

import (
	"context"

	"github.com/keshon/beacon/internal/monitor"
	"github.com/keshon/beacon/internal/store"
)

// MonitorSource lists monitors to check and resolves definitions at execution time.
type MonitorSource interface {
	List(ctx context.Context) ([]*monitor.Monitor, error)
	Resolve(id string) (*monitor.Monitor, error)
	// RequireLocalMonitor is true when check results must have a local monitors.json entry.
	RequireLocalMonitor(id string) bool
}

// LocalSource uses only monitors defined in the local store.
type LocalSource struct {
	Store *store.Store
}

func (l LocalSource) List(ctx context.Context) ([]*monitor.Monitor, error) {
	_ = ctx
	return l.Store.GetMonitors()
}

func (l LocalSource) Resolve(id string) (*monitor.Monitor, error) {
	return l.Store.GetMonitor(id)
}

func (l LocalSource) RequireLocalMonitor(id string) bool {
	_ = id
	return true
}
