package command

import (
	"io"

	"github.com/keshon/beacon/internal/store"
)

// CLIContext is passed via command.Invocation.Data for CLI commands.
type CLIContext struct {
	Store *store.Store
	Out   io.Writer
}
