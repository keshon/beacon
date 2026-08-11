package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/keshon/datastore"

	"github.com/keshon/beacon/internal/monitor"
)

// Import from the legacy JSON files.
//
// Before the datastore each collection lived in its own file shaped
// {"<key>": <data>}. Those files are NOT deleted: the import reads them
// and leaves them where they are, so rolling back is putting the old
// binary back.
//
// The import runs once, and what guarantees that is a marker record
// rather than an "are the collections empty" test. The difference
// matters: a user who deleted every monitor would otherwise get them all
// back on the next start.

const legacyMarkKey = "legacy-import"

type markRec struct {
	Name string    `json:"name"`
	At   time.Time `json:"at"`
	Note string    `json:"note"`
}

func (m *markRec) Key() string { return m.Name }

func (s *Store) importLegacy() error {
	if _, done := s.marks.Get(legacyMarkKey); done {
		return nil
	}

	var (
		monitors map[string]*monitor.Monitor
		states   map[string]*monitor.MonitorState
		events   []CheckRecord
		cfg      json.RawMessage
		peers    map[string]*PeerData
	)
	files := []struct {
		name string
		key  string
		dest any
	}{
		{"monitors.json", "monitors", &monitors},
		{"state.json", "state", &states},
		{"events.json", "events", &events},
		{"config.json", "config", &cfg},
		{"peer_data.json", "peer_data", &peers},
	}

	found := false
	for _, f := range files {
		ok, err := loadWrapped(filepath.Join(s.dataDir, f.name), f.key, f.dest)
		if err != nil {
			return err
		}
		found = found || ok
	}
	if !found {
		// Fresh install: mark it so we stop looking.
		return s.marks.Put(&markRec{Name: legacyMarkKey, At: time.Now(), Note: "nothing to import"})
	}

	// One transaction for everything: either the whole of the old data lands
	// in the new store, or nothing does and no marker is written — in which
	// case the next start tries again.
	err := s.db.Update(func(tx *datastore.Tx) error {
		mons := datastore.In(tx, s.monitors)
		for _, m := range monitors {
			if m == nil || m.ID == "" {
				continue
			}
			if err := mons.Put(&monitorRec{M: m}); err != nil {
				return err
			}
		}

		sts := datastore.In(tx, s.state)
		for _, st := range states {
			if st == nil || st.MonitorID == "" {
				continue
			}
			if err := sts.Put(&stateRec{S: st}); err != nil {
				return err
			}
		}

		// The old ring held up to ten thousand records across all monitors; the
		// new rule is N per monitor, so take each monitor's own tail.
		checks := datastore.In(tx, s.checks)
		perMonitor := map[string][]CheckRecord{}
		for _, rec := range events {
			if rec.MonitorID == "" {
				continue
			}
			perMonitor[rec.MonitorID] = append(perMonitor[rec.MonitorID], rec)
		}
		for _, list := range perMonitor {
			if len(list) > uptimeIndexLimit {
				list = list[len(list)-uptimeIndexLimit:]
			}
			for i := range list {
				cp := list[i]
				if err := checks.Put(&cp); err != nil {
					return err
				}
			}
		}

		// Rebuild the buckets from the imported samples so the history is
		// visible immediately instead of accumulating from zero.
		if err := s.buildRollupsTx(tx, events); err != nil {
			return err
		}

		if len(cfg) > 0 {
			if err := datastore.In(tx, s.config).Put(&configRec{Raw: cfg}); err != nil {
				return err
			}
		}

		prs := datastore.In(tx, s.peers)
		for _, pd := range peers {
			if pd == nil || pd.NodeID == "" {
				continue
			}
			if err := prs.Put(pd); err != nil {
				return err
			}
		}

		return datastore.In(tx, s.marks).Put(&markRec{
			Name: legacyMarkKey,
			At:   time.Now(),
			Note: fmt.Sprintf("monitors %d, states %d, checks %d, peers %d",
				len(monitors), len(states), len(events), len(peers)),
		})
	})
	if err != nil {
		return fmt.Errorf("import legacy data: %w", err)
	}
	return nil
}

// loadWrapped reads {"<key>": <value>} from path into dest. Reports whether the
// file held anything; a missing file is not an error.
func loadWrapped(path, key string, dest any) (bool, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read %q: %w", path, err)
	}
	if len(raw) == 0 {
		return false, nil
	}
	var wrap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return false, fmt.Errorf("parse %q: %w", path, err)
	}
	v, ok := wrap[key]
	if !ok || len(v) == 0 || string(v) == "null" {
		return false, nil
	}
	if err := json.Unmarshal(v, dest); err != nil {
		return false, fmt.Errorf("parse %q key %q: %w", path, key, err)
	}
	return true, nil
}
