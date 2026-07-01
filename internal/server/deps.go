package server

import (
	"github.com/keshon/beacon/internal/cluster"
	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/monitor/scheduler"
	"github.com/keshon/beacon/internal/notify"
	"github.com/keshon/beacon/internal/server/middleware"
	"github.com/keshon/beacon/internal/server/stream"
	"github.com/keshon/beacon/internal/storage"
)

type Deps struct {
	Store     *storage.Store
	Cfg       *config.Config
	Scheduler *scheduler.Scheduler
	Cluster   *cluster.Runtime
	StreamHub *stream.Hub
	Auth      *middleware.Auth
	TplDir    string
	StaticDir string
	TestLimit *notify.RateLimiter
}
