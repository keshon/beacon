package storage

import (
	"sort"
	"strconv"
	"time"

	"github.com/keshon/datastore"
)

// Hourly rollups.
//
// Raw samples last hours; the interface asks for days and months. Keeping a
// month of raw is not an option: the datastore holds everything in memory and
// says so plainly. Nineteen monitors at a thirty-second interval are one and a
// half million records a month.
//
// So every check lands in two records at once: the raw one (lives hours) and
// its hourly bucket (lives months). Buckets are 19 × 24 × 90 ≈ 41 thousand
// records a quarter, which is nothing.
//
// Both are written in the SAME transaction as the monitor state. A rollup that
// drifted from the history would lie more convincingly the longer it lived.

// Rollup is one monitor's outcome over one hour.
type Rollup struct {
	MonitorID string `json:"monitor_id"`
	// Hour is the start of the hour in Unix seconds, always a multiple of 3600.
	Hour     int64 `json:"hour"`
	Total    int   `json:"total"`
	Failed   int   `json:"failed"`
	LatSumMs int64 `json:"lat_sum_ms"`
	LatMaxMs int64 `json:"lat_max_ms"`
}

func (r *Rollup) Key() string {
	return r.MonitorID + "/" + strconv.FormatInt(r.Hour, 10)
}

// Ok is how many checks in the hour passed.
func (r *Rollup) Ok() int { return r.Total - r.Failed }

// AvgLatency is the mean over the hour. No checks yields zero, not a panic.
func (r *Rollup) AvgLatency() time.Duration {
	if r.Total == 0 {
		return 0
	}
	return time.Duration(r.LatSumMs/int64(r.Total)) * time.Millisecond
}

const (
	// rawMaxAge caps how old a raw sample may get. It works TOGETHER with the
	// per-monitor count window: for a monitor checked every five minutes five
	// hundred samples are almost two days and there is no reason to cut them;
	// for one checked every thirty seconds the count bound bites first.
	rawMaxAge = 7 * 24 * time.Hour

	// rollupMaxAge is how long hourly buckets live.
	rollupMaxAge = 90 * 24 * time.Hour
)

// hourOf returns the start of the hour a moment belongs to.
func hourOf(t time.Time) int64 { return t.UTC().Truncate(time.Hour).Unix() }

// rollUpTx folds one check into its hourly bucket inside the caller's transaction.
func (s *Store) rollUpTx(tx *datastore.Tx, rec CheckRecord) error {
	rolls := datastore.In(tx, s.rollups)
	hour := hourOf(rec.Time)
	key := rec.MonitorID + "/" + strconv.FormatInt(hour, 10)

	r, ok := rolls.Get(key)
	if !ok {
		r = &Rollup{MonitorID: rec.MonitorID, Hour: hour}
	}
	r.Total++
	if !rec.Success {
		r.Failed++
	}
	ms := rec.Latency.Milliseconds()
	r.LatSumMs += ms
	if ms > r.LatMaxMs {
		r.LatMaxMs = ms
	}
	return rolls.Put(r)
}

// pruneRollupsTx drops buckets past the age cap. The sorted index on the hour
// makes that a range, not a scan of everything.
func (s *Store) pruneRollupsTx(tx *datastore.Tx, now time.Time) error {
	cutoff := now.Add(-rollupMaxAge).Unix()
	old := datastore.InSorted(tx, s.rollupsByHour).Range(0, cutoff)
	if len(old) == 0 {
		return nil
	}
	rolls := datastore.In(tx, s.rollups)
	for _, r := range old {
		if err := rolls.Delete(r.Key()); err != nil {
			return err
		}
	}
	return nil
}

// GetRollups returns a monitor's hourly buckets over [from, to], oldest first.
//
// Empty hours are NOT invented: their absence is a fact of its own ("no checks
// ran"), and the application must be able to tell it from "an hour passed
// without failures".
func (s *Store) GetRollups(monitorID string, from, to time.Time) ([]Rollup, error) {
	all := s.rollupsByHour.Range(hourOf(from), hourOf(to))
	out := make([]Rollup, 0, len(all))
	for _, r := range all {
		if r.MonitorID == monitorID {
			out = append(out, *r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Hour < out[j].Hour })
	return out, nil
}

// GetRollupsBatch is the same for many monitors at once: the list screen reads
// every row's history in one pass instead of one pass per row.
func (s *Store) GetRollupsBatch(monitorIDs []string, from, to time.Time) (map[string][]Rollup, error) {
	want := make(map[string]struct{}, len(monitorIDs))
	for _, id := range monitorIDs {
		if id != "" {
			want[id] = struct{}{}
		}
	}
	out := make(map[string][]Rollup, len(want))
	if len(want) == 0 {
		return out, nil
	}
	for _, r := range s.rollupsByHour.Range(hourOf(from), hourOf(to)) {
		if _, ok := want[r.MonitorID]; ok {
			out[r.MonitorID] = append(out[r.MonitorID], *r)
		}
	}
	for id := range out {
		list := out[id]
		sort.Slice(list, func(i, j int) bool { return list[i].Hour < list[j].Hour })
		out[id] = list
	}
	return out, nil
}

// buildRollupsTx rebuilds buckets from a batch of raw records. The legacy
// import needs it: history has to be visible right after the move, not
// accumulate from zero.
func (s *Store) buildRollupsTx(tx *datastore.Tx, records []CheckRecord) error {
	agg := map[string]*Rollup{}
	for _, rec := range records {
		if rec.MonitorID == "" {
			continue
		}
		hour := hourOf(rec.Time)
		key := rec.MonitorID + "/" + strconv.FormatInt(hour, 10)
		r, ok := agg[key]
		if !ok {
			r = &Rollup{MonitorID: rec.MonitorID, Hour: hour}
			agg[key] = r
		}
		r.Total++
		if !rec.Success {
			r.Failed++
		}
		ms := rec.Latency.Milliseconds()
		r.LatSumMs += ms
		if ms > r.LatMaxMs {
			r.LatMaxMs = ms
		}
	}
	rolls := datastore.In(tx, s.rollups)
	for _, r := range agg {
		if err := rolls.Put(r); err != nil {
			return err
		}
	}
	return nil
}
