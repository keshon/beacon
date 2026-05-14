package commands

import (
	"io"

	"github.com/keshon/commandkit"
)

// CLIData carries CLI context for command execution.
type CLIData struct {
	Out  io.Writer
	Args map[string]string // name, type, target, id, enabled
}

func getCLIData(inv *commandkit.Invocation) *CLIData {
	if inv == nil || inv.Data == nil {
		return nil
	}
	d, ok := inv.Data.(*CLIData)
	if !ok {
		return nil
	}
	return d
}
