// Package storage keeps Beacon's data: monitors, their state, the outcomes of
// individual checks, peer caches and the config blob.
//
// The engine underneath is github.com/keshon/datastore: everything lives in
// memory, every commit is appended to a write-ahead log and fsynced before it
// is acknowledged. That replaced hand-rolled JSON files, and it replaced them
// for three concrete reasons, not for novelty.
//
//   - Appending ONE check outcome used to rewrite the WHOLE events file. At
//     nineteen monitors and a thirty-second interval that was a ten-thousand
//     record JSON dump roughly every second and a half.
//   - Recording a check wrote two files in turn — events, then state. A crash
//     between them left the two disagreeing, and nothing noticed.
//   - History was two ring buffers in memory (ten thousand global, five hundred
//     per monitor) with the same records in both.
//
// Now a check outcome and the state it produced commit as ONE transaction, and
// the log grows by one framed record.
//
// The public API of Store is unchanged: the callers outside this package never
// learned which engine is underneath, and that is the point of the seam.
package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/keshon/datastore"

	"github.com/keshon/beacon/internal/monitor"
)

var ErrMonitorNotFound = errors.New("monitor not found")

// CheckRecord is one persisted outcome of a monitor probe (uptime history sample).
type CheckRecord struct {
	MonitorID string        `json:"monitor_id"`
	Success   bool          `json:"success"`
	Time      time.Time     `json:"time"`
	Latency   time.Duration `json:"latency"`
	Error     string        `json:"error"`
}

// Key addresses the record in the log. Monitor plus nanosecond: a single
// monitor cannot produce two outcomes in the same nanosecond, and prefixing by
// monitor keeps a monitor's samples adjacent in the snapshot.
func (r *CheckRecord) Key() string {
	return r.MonitorID + "/" + strconv.FormatInt(r.Time.UnixNano(), 10)
}

// PeerData holds synced data from a peer node.
type PeerData struct {
	NodeID       string                           `json:"node_id"`
	PeerURL      string                           `json:"peer_url,omitempty"`
	Monitors     []*monitor.Monitor               `json:"monitors"`
	State        map[string]*monitor.MonitorState `json:"state"`
	LastSeen     time.Time                        `json:"last_seen"`
	LastExport   time.Time                        `json:"last_export,omitempty"`
	LastError    string                           `json:"last_error,omitempty"`
	SyncWarnings []string                         `json:"sync_warnings,omitempty"`
}

func (p *PeerData) Key() string { return p.NodeID }

// The domain types live in package monitor and must not learn about storage,
// so they are wrapped here rather than given a Key method there.
type monitorRec struct {
	M *monitor.Monitor `json:"m"`
}

func (r *monitorRec) Key() string { return r.M.ID }

type stateRec struct {
	S *monitor.MonitorState `json:"s"`
}

func (r *stateRec) Key() string { return r.S.MonitorID }

// The config is one opaque JSON blob owned by package config; storage only
// carries it. One record in a collection of one.
type configRec struct {
	Raw json.RawMessage `json:"raw"`
}

func (r *configRec) Key() string { return configKey }

const configKey = "config"

// uptimeIndexLimit is how many outcomes are kept PER MONITOR.
//
// The old store kept two limits at once — five hundred per monitor and ten
// thousand across all of them — and the global one could starve a busy monitor
// out of its own history. One rule now: every monitor keeps its own last N,
// whatever the neighbours do.
//
// At a thirty-second interval this is four hours. That is not enough for the
// day-long history the interface wants, and raising it is a deliberate
// decision about memory, not a constant to nudge: see the note on rollups in
// the project plan.
const uptimeIndexLimit = 500

// Store is the whole persistence layer. Reads hand out copies, never pointers
// into the store.
type Store struct {
	db *datastore.DB

	monitors *datastore.Collection[*monitorRec]
	state    *datastore.Collection[*stateRec]
	checks   *datastore.Collection[*CheckRecord]
	peers    *datastore.Collection[*PeerData]
	config   *datastore.Collection[*configRec]
	marks    *datastore.Collection[*markRec]
	rollups  *datastore.Collection[*Rollup]

	incidents  *datastore.Collection[*Incident]
	deliveries *datastore.Collection[*Delivery]
	mutes      *datastore.Collection[*Mute]

	checksByMonitor    *datastore.Index[*CheckRecord]
	checksByTime       *datastore.SortedIndex[*CheckRecord]
	rollupsByHour      *datastore.SortedIndex[*Rollup]
	incidentsByMonitor *datastore.Index[*Incident]
	incidentsByStart   *datastore.SortedIndex[*Incident]
	deliveriesByTime   *datastore.SortedIndex[*Delivery]

	dataDir string
}

func New(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("data dir: %w", err)
	}

	s := &Store{dataDir: dataDir}
	s.db = datastore.New(datastore.Options{Dir: filepath.Join(dataDir, "db")})

	s.monitors = datastore.Register[*monitorRec](s.db, "monitors")
	s.state = datastore.Register[*stateRec](s.db, "state")
	s.checks = datastore.Register[*CheckRecord](s.db, "checks")
	s.peers = datastore.Register[*PeerData](s.db, "peers")
	s.config = datastore.Register[*configRec](s.db, "config")
	s.marks = datastore.Register[*markRec](s.db, "marks")
	s.rollups = datastore.Register[*Rollup](s.db, "rollups")
	s.incidents = datastore.Register[*Incident](s.db, "incidents")
	s.deliveries = datastore.Register[*Delivery](s.db, "deliveries")
	s.mutes = datastore.Register[*Mute](s.db, "mutes")

	// Collections and indexes must be declared before Open: replaying the log
	// rebuilds the indexes, so it has to know they exist.
	s.checksByMonitor = datastore.AddIndex(s.checks, "monitor",
		func(r *CheckRecord) []string { return []string{r.MonitorID} })
	s.checksByTime = datastore.AddSorted(s.checks, "time",
		func(r *CheckRecord) int64 { return r.Time.UnixNano() })
	s.rollupsByHour = datastore.AddSorted(s.rollups, "hour",
		func(r *Rollup) int64 { return r.Hour })
	s.incidentsByMonitor = datastore.AddIndex(s.incidents, "monitor",
		func(i *Incident) []string { return []string{i.MonitorID} })
	s.incidentsByStart = datastore.AddSorted(s.incidents, "start",
		func(i *Incident) int64 { return i.StartedAt.UnixNano() })
	s.deliveriesByTime = datastore.AddSorted(s.deliveries, "at",
		func(d *Delivery) int64 { return d.At.UnixNano() })

	if err := s.db.Open(); err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}

	if err := s.importLegacy(); err != nil {
		s.db.Close()
		return nil, err
	}
	if err := s.markIncidentsStart(); err != nil {
		s.db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the database and its directory lock.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Ping verifies the data directory is still accessible.
func (s *Store) Ping() error {
	if _, err := os.Stat(s.dataDir); err != nil {
		return fmt.Errorf("data dir: %w", err)
	}
	return nil
}

// ── Monitors ──────────────────────────────────────────────────────────────

func (s *Store) GetMonitors() ([]*monitor.Monitor, error) {
	list := make([]*monitor.Monitor, 0, s.monitors.Len())
	for rec := range s.monitors.All() {
		list = append(list, rec.M)
	}
	if len(list) == 0 {
		return nil, nil
	}
	sortMonitorsByName(list)
	return list, nil
}

func sortMonitorsByName(list []*monitor.Monitor) {
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
}

func (s *Store) GetMonitor(id string) (*monitor.Monitor, error) {
	rec, ok := s.monitors.Get(id)
	if !ok {
		return nil, nil
	}
	return rec.M, nil
}

func (s *Store) SetMonitor(mon *monitor.Monitor) error {
	return s.monitors.Put(&monitorRec{M: mon})
}

// MonitorExists reports whether a monitor ID is defined locally.
func (s *Store) MonitorExists(id string) (bool, error) {
	_, ok := s.monitors.Get(id)
	return ok, nil
}

// DeleteMonitor removes the monitor together with everything that belonged to
// it — its state and every check outcome — as one transaction. Half a deletion
// was possible before and left orphaned samples behind.
func (s *Store) DeleteMonitor(id string) error {
	return s.db.Update(func(tx *datastore.Tx) error {
		mons := datastore.In(tx, s.monitors)
		if _, ok := mons.Get(id); !ok {
			return ErrMonitorNotFound
		}
		if err := mons.Delete(id); err != nil {
			return err
		}
		// Removing an absent key is not an error in the datastore, so a monitor
		// that never ran needs no special case here.
		if err := datastore.In(tx, s.state).Delete(id); err != nil {
			return err
		}
		checks := datastore.In(tx, s.checks)
		for _, rec := range datastore.InIndex(tx, s.checksByMonitor).Find(id) {
			if err := checks.Delete(rec.Key()); err != nil {
				return err
			}
		}
		incs := datastore.In(tx, s.incidents)
		for _, inc := range datastore.InIndex(tx, s.incidentsByMonitor).Find(id) {
			if err := incs.Delete(inc.Key()); err != nil {
				return err
			}
		}
		// The hourly buckets go too. They have no index by monitor — the sorted
		// one is by hour, which is what every read needs — so this walks the
		// keys. A deleted monitor is rare; a stale bucket would outlive it and
		// quietly inflate a summary.
		rolls := datastore.In(tx, s.rollups)
		prefix := id + "/"
		for _, key := range rolls.Keys() {
			if strings.HasPrefix(key, prefix) {
				if err := rolls.Delete(key); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// UpdateMonitor applies fn to the stored monitor and writes the result back.
// Read and write are one transaction, so a concurrent writer cannot slip
// between them.
func (s *Store) UpdateMonitor(id string, fn func(*monitor.Monitor) error) (*monitor.Monitor, error) {
	var out *monitor.Monitor
	err := s.db.Update(func(tx *datastore.Tx) error {
		mons := datastore.In(tx, s.monitors)
		rec, ok := mons.Get(id)
		if !ok {
			return ErrMonitorNotFound
		}
		work := rec.M
		if err := fn(work); err != nil {
			return err
		}
		if err := mons.Put(&monitorRec{M: work}); err != nil {
			return err
		}
		out = work
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ── State ─────────────────────────────────────────────────────────────────

func (s *Store) GetState(monitorID string) (*monitor.MonitorState, error) {
	rec, ok := s.state.Get(monitorID)
	if !ok {
		return nil, nil
	}
	return rec.S, nil
}

func (s *Store) GetAllState() (map[string]*monitor.MonitorState, error) {
	if s.state.Len() == 0 {
		return nil, nil
	}
	out := make(map[string]*monitor.MonitorState, s.state.Len())
	for rec := range s.state.All() {
		out[rec.S.MonitorID] = rec.S
	}
	return out, nil
}

func (s *Store) SetState(st *monitor.MonitorState) error {
	return s.state.Put(&stateRec{S: st})
}

// ── Checks ────────────────────────────────────────────────────────────────

func (s *Store) AppendCheckRecord(rec CheckRecord) error {
	return s.db.Update(func(tx *datastore.Tx) error {
		return s.appendCheckTx(tx, rec)
	})
}

// RecordCheckResult stores the outcome and the state it produced as ONE
// transaction. Previously these were two file writes in a row, and a crash
// between them left the history and the state disagreeing.
func (s *Store) RecordCheckResult(rec CheckRecord, st *monitor.MonitorState, requireLocalMonitor bool) error {
	return s.db.Update(func(tx *datastore.Tx) error {
		if requireLocalMonitor {
			if _, ok := datastore.In(tx, s.monitors).Get(rec.MonitorID); !ok {
				return ErrMonitorNotFound
			}
		}
		if err := s.appendCheckTx(tx, rec); err != nil {
			return err
		}
		if err := s.trackIncidentTx(tx, rec, st); err != nil {
			return err
		}
		return datastore.In(tx, s.state).Put(&stateRec{S: st})
	})
}

// appendCheckTx records one outcome inside the caller's transaction: the raw
// sample, its hourly bucket, and whatever retention drops as a result.
//
// All three belong to one commit. A rollup that survived while its samples did
// not — or the other way round — would be a lie that outlives the truth.
func (s *Store) appendCheckTx(tx *datastore.Tx, rec CheckRecord) error {
	checks := datastore.In(tx, s.checks)
	cp := rec
	if err := checks.Put(&cp); err != nil {
		return err
	}
	if err := s.rollUpTx(tx, rec); err != nil {
		return err
	}
	return s.pruneTx(tx, rec.MonitorID, rec.Time)
}

// pruneTx enforces both bounds on raw samples and the age bound on rollups.
//
// Two bounds on raw, not one, and they protect different monitors. The count
// keeps a monitor checked every thirty seconds from filling memory; the age
// keeps a monitor checked once an hour from holding samples from last spring.
// Whichever bites first wins.
func (s *Store) pruneTx(tx *datastore.Tx, monitorID string, now time.Time) error {
	checks := datastore.In(tx, s.checks)
	kept := datastore.InIndex(tx, s.checksByMonitor).Find(monitorID)
	sortByTime(kept)

	drop := 0
	if len(kept) > uptimeIndexLimit {
		drop = len(kept) - uptimeIndexLimit
	}
	cutoff := now.Add(-rawMaxAge)
	for drop < len(kept) && kept[drop].Time.Before(cutoff) {
		drop++
	}
	for _, old := range kept[:drop] {
		if err := checks.Delete(old.Key()); err != nil {
			return err
		}
	}
	if err := s.pruneRollupsTx(tx, now); err != nil {
		return err
	}
	return s.pruneIncidentsTx(tx, now)
}

func sortByTime(list []*CheckRecord) {
	sort.Slice(list, func(i, j int) bool { return list[i].Time.Before(list[j].Time) })
}

// GetCheckRecords returns the most recent outcomes across all monitors, oldest
// first — the shape the callers already expected.
func (s *Store) GetCheckRecords(limit int) ([]CheckRecord, error) {
	total := s.checksByTime.Len()
	if total == 0 {
		return nil, nil
	}
	if limit <= 0 || limit > total {
		limit = total
	}
	// Desc gives newest first; the contract is oldest first.
	recent := s.checksByTime.Desc(limit, 0)
	out := make([]CheckRecord, len(recent))
	for i, r := range recent {
		out[len(recent)-1-i] = *r
	}
	return out, nil
}

// GetUptimeSamples returns the last limit check outcomes for monitorID, oldest first.
func (s *Store) GetUptimeSamples(monitorID string, limit int) ([]CheckRecord, error) {
	return s.samples(monitorID, limit), nil
}

// GetUptimeSamplesBatch returns the last limit check outcomes for each monitor ID, oldest first.
func (s *Store) GetUptimeSamplesBatch(monitorIDs []string, limit int) (map[string][]CheckRecord, error) {
	out := make(map[string][]CheckRecord, len(monitorIDs))
	seen := make(map[string]struct{}, len(monitorIDs))
	for _, id := range monitorIDs {
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out[id] = s.samples(id, limit)
	}
	return out, nil
}

func (s *Store) samples(monitorID string, limit int) []CheckRecord {
	if limit <= 0 {
		limit = 120
	}
	if limit > uptimeIndexLimit {
		limit = uptimeIndexLimit
	}
	found := s.checksByMonitor.Find(monitorID)
	if len(found) == 0 {
		return nil
	}
	sortByTime(found)
	if limit > len(found) {
		limit = len(found)
	}
	tail := found[len(found)-limit:]
	out := make([]CheckRecord, len(tail))
	for i, r := range tail {
		out[i] = *r
	}
	return out
}

// ── Config ────────────────────────────────────────────────────────────────

func (s *Store) GetConfig(dest any) (bool, error) {
	rec, ok := s.config.Get(configKey)
	if !ok || len(rec.Raw) == 0 {
		return false, nil
	}
	if err := json.Unmarshal(rec.Raw, dest); err != nil {
		return false, fmt.Errorf("read config: %w", err)
	}
	return true, nil
}

func (s *Store) SetConfig(cfg any) error {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return s.config.Put(&configRec{Raw: raw})
}

// ── Peers ─────────────────────────────────────────────────────────────────

func (s *Store) GetPeerData(nodeID string) (*PeerData, error) {
	pd, ok := s.peers.Get(nodeID)
	if !ok {
		return nil, nil
	}
	return pd, nil
}

func (s *Store) SetPeerData(data *PeerData) error {
	return s.peers.Put(data)
}

// SetPeerDataError records a sync error for the peer URL without replacing cached data.
func (s *Store) SetPeerDataError(peerURL, errMsg string) error {
	normalized := strings.TrimSuffix(peerURL, "/")
	return s.db.Update(func(tx *datastore.Tx) error {
		peers := datastore.In(tx, s.peers)
		for _, key := range peers.Keys() {
			pd, ok := peers.Get(key)
			if !ok {
				continue
			}
			if strings.TrimSuffix(pd.PeerURL, "/") == normalized {
				pd.LastError = errMsg
				return peers.Put(pd)
			}
		}
		return nil
	})
}

// PrunePeerData removes cache entries whose peer URL is not in configuredPeers.
func (s *Store) PrunePeerData(configuredPeers []string) error {
	allowed := make(map[string]struct{}, len(configuredPeers))
	for _, u := range configuredPeers {
		u = strings.TrimSuffix(strings.TrimSpace(u), "/")
		if u != "" {
			allowed[u] = struct{}{}
		}
	}
	return s.db.Update(func(tx *datastore.Tx) error {
		peers := datastore.In(tx, s.peers)
		for _, nodeID := range peers.Keys() {
			pd, ok := peers.Get(nodeID)
			if !ok {
				continue
			}
			key := strings.TrimSuffix(pd.PeerURL, "/")
			if key == "" {
				key = nodeID
			}
			if _, ok := allowed[key]; ok {
				continue
			}
			if err := peers.Delete(nodeID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) GetAllPeerData() (map[string]*PeerData, error) {
	if s.peers.Len() == 0 {
		return nil, nil
	}
	out := make(map[string]*PeerData, s.peers.Len())
	for pd := range s.peers.All() {
		out[pd.NodeID] = pd
	}
	return out, nil
}

// ── Export snapshot ───────────────────────────────────────────────────────

// ExportSnapshot is a consistent monitors+state read for sync export.
type ExportSnapshot struct {
	Monitors []*monitor.Monitor
	State    map[string]*monitor.MonitorState
}

// GetExportSnapshot returns monitors and state read together, so the pair
// cannot straddle a write.
func (s *Store) GetExportSnapshot() (ExportSnapshot, error) {
	var snap ExportSnapshot
	err := s.db.View(func(tx *datastore.Tx) error {
		mons := datastore.In(tx, s.monitors)
		states := datastore.In(tx, s.state)

		for _, id := range mons.Keys() {
			rec, ok := mons.Get(id)
			if !ok {
				continue
			}
			snap.Monitors = append(snap.Monitors, rec.M)
		}
		sortMonitorsByName(snap.Monitors)

		snap.State = make(map[string]*monitor.MonitorState)
		for _, id := range states.Keys() {
			// Drop state entries with no matching local monitor.
			if _, ok := mons.Get(id); !ok {
				continue
			}
			st, ok := states.Get(id)
			if !ok {
				continue
			}
			snap.State[id] = st.S
		}
		return nil
	})
	return snap, err
}
