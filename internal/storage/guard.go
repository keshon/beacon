package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// Refusal to start on a data directory this build cannot read.
//
// Beacon used to keep each collection in its own JSON file and imported them
// into the store on first open. The importer is gone: every deployment that
// needed it has been converted, and carrying a one-time migration forever
// means carrying its bugs forever too.
//
// What the removal leaves behind is a quiet failure. Point this binary at an
// old data directory and nothing complains — the store simply finds no `db/`,
// creates an empty one beside the untouched JSON, and comes up with zero
// monitors. On a laptop that is a puzzle; on the machine that is supposed to
// be watching production it is an outage nobody is told about, because the
// thing that reports outages is the thing that came up empty.
//
// So it is named out loud instead. The check is narrow on purpose: it fires
// only when the directory clearly holds the OLD shape and just as clearly
// holds none of the new one. A converted directory that still has the old
// files lying around starts normally, because its `db/` is there.

// legacyFiles are the collection files the pre-datastore versions wrote.
var legacyFiles = []string{
	"monitors.json",
	"state.json",
	"events.json",
	"peer_data.json",
}

func (s *Store) guardUnconverted() error {
	dbDir := filepath.Join(s.dataDir, "db")
	if entries, err := os.ReadDir(dbDir); err == nil && len(entries) > 0 {
		return nil // the new shape is present; nothing to warn about
	}

	found := make([]string, 0, len(legacyFiles))
	for _, name := range legacyFiles {
		info, err := os.Stat(filepath.Join(s.dataDir, name))
		if err == nil && info.Size() > 0 {
			found = append(found, name)
		}
	}
	if len(found) == 0 {
		return nil // a genuinely fresh directory
	}

	return fmt.Errorf(
		"data directory %q holds the old JSON format (%v) and no db/ — this build cannot read it.\n"+
			"Starting anyway would come up with zero monitors and report nothing wrong.\n"+
			"Convert the directory with a build that still has the importer, then copy its db/ here",
		s.dataDir, found)
}
