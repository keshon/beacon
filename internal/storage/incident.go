package storage

import (
	"sort"
	"strconv"
	"time"

	"github.com/keshon/datastore"

	"github.com/keshon/beacon/internal/monitor"
)

// Incidents.
//
// A stretch of time during which a monitor was down. Until now this was not
// stored at all: the application fired a notification on the way down, another
// on the way up, and kept nothing. "How long was it out" and "is this the third
// time this week" had no answer anywhere.
//
// An incident opens when the monitor is DECLARED down and closes when it comes
// back — it follows the state, not the individual check. That matters because a
// monitor with retries survives a single failed probe without being down, and
// an incident per failed probe would be noise rather than history. The rule
// here is the same one that decides whether to notify, so the incident log and
// the notifications tell the same story.
//
// Nothing is backfilled from the imported samples, deliberately. Rollups are
// aggregates and rebuilding them changes no meaning; an incident is a discrete
// event that says "this happened and we alerted". Inventing them retroactively
// would put things on the incidents screen that never occurred.
type Incident struct {
	MonitorID string    `json:"monitor_id"`
	StartedAt time.Time `json:"started_at"`
	// EndedAt is zero while the incident is still running.
	EndedAt time.Time `json:"ended_at,omitempty"`
	// Reason is the error that opened it — the first one seen, not the last:
	// what broke is more useful than what it looked like on the way out.
	Reason string `json:"reason,omitempty"`
	// Checks counts the failed probes inside the incident.
	Checks int `json:"checks"`
}

func (i *Incident) Key() string {
	return i.MonitorID + "/" + strconv.FormatInt(i.StartedAt.UnixNano(), 10)
}

// Ongoing reports whether the incident is still running.
func (i *Incident) Ongoing() bool { return i.EndedAt.IsZero() }

// Duration is how long it lasted, or how long it has been running so far.
func (i *Incident) Duration(now time.Time) time.Duration {
	if i.Ongoing() {
		return now.Sub(i.StartedAt)
	}
	return i.EndedAt.Sub(i.StartedAt)
}

// incidentMaxAge is how long a closed incident is kept. They are small and
// rare; the horizon is generous on purpose, because "has this happened before"
// is a question people ask about last season, not last week.
const incidentMaxAge = 365 * 24 * time.Hour

// trackIncidentTx opens or closes an incident according to the state the check
// produced, inside the caller's transaction.
func (s *Store) trackIncidentTx(tx *datastore.Tx, rec CheckRecord, st *monitor.MonitorState) error {
	if st == nil {
		return nil
	}
	incs := datastore.In(tx, s.incidents)
	open := s.openIncidentTx(tx, rec.MonitorID)

	switch st.Status {
	case monitor.StatusDown:
		if open == nil {
			return incs.Put(&Incident{
				MonitorID: rec.MonitorID,
				StartedAt: rec.Time,
				Reason:    rec.Error,
				Checks:    1,
			})
		}
		open.Checks++
		return incs.Put(open)

	case monitor.StatusUp:
		if open == nil {
			return nil
		}
		open.EndedAt = rec.Time
		return incs.Put(open)
	}
	// StatusUnknown leaves an open incident alone: not knowing is not recovery.
	return nil
}

// openIncidentTx returns the monitor's running incident, if it has one.
func (s *Store) openIncidentTx(tx *datastore.Tx, monitorID string) *Incident {
	var newest *Incident
	for _, inc := range datastore.InIndex(tx, s.incidentsByMonitor).Find(monitorID) {
		if !inc.Ongoing() {
			continue
		}
		if newest == nil || inc.StartedAt.After(newest.StartedAt) {
			newest = inc
		}
	}
	return newest
}

// pruneIncidentsTx drops closed incidents past the horizon. Running ones are
// never dropped, however long they have been running: a monitor that has been
// down for a year is a fact, not litter.
func (s *Store) pruneIncidentsTx(tx *datastore.Tx, now time.Time) error {
	cutoff := now.Add(-incidentMaxAge).UnixNano()
	old := datastore.InSorted(tx, s.incidentsByStart).Range(0, cutoff)
	if len(old) == 0 {
		return nil
	}
	incs := datastore.In(tx, s.incidents)
	for _, inc := range old {
		if inc.Ongoing() {
			continue
		}
		if err := incs.Delete(inc.Key()); err != nil {
			return err
		}
	}
	return nil
}

// GetIncidents returns incidents across all monitors that started within
// [from, to], newest first — the order the incidents screen reads them in.
func (s *Store) GetIncidents(from, to time.Time, limit int) ([]Incident, error) {
	found := s.incidentsByStart.Range(from.UnixNano(), to.UnixNano())
	sort.Slice(found, func(i, j int) bool {
		return found[i].StartedAt.After(found[j].StartedAt)
	})
	if limit > 0 && limit < len(found) {
		found = found[:limit]
	}
	out := make([]Incident, len(found))
	for i, inc := range found {
		out[i] = *inc
	}
	return out, nil
}

// GetMonitorIncidents returns one monitor's incidents within [from, to],
// newest first.
func (s *Store) GetMonitorIncidents(monitorID string, from, to time.Time) ([]Incident, error) {
	var out []Incident
	lo, hi := from.UnixNano(), to.UnixNano()
	for _, inc := range s.incidentsByMonitor.Find(monitorID) {
		n := inc.StartedAt.UnixNano()
		if n < lo || n > hi {
			continue
		}
		out = append(out, *inc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	return out, nil
}

// CountIncidents is how many incidents a monitor had since a moment — what
// turns "it is down" into "it is down for the third time this week".
func (s *Store) CountIncidents(monitorID string, since time.Time) (int, error) {
	n := 0
	for _, inc := range s.incidentsByMonitor.Find(monitorID) {
		if !inc.StartedAt.Before(since) {
			n++
		}
	}
	return n, nil
}

// CountIncidentsBatch answers the same question for many monitors at once, so
// the list screen does not ask it once per row.
func (s *Store) CountIncidentsBatch(monitorIDs []string, since time.Time) (map[string]int, error) {
	out := make(map[string]int, len(monitorIDs))
	for _, id := range monitorIDs {
		if id == "" {
			continue
		}
		if _, done := out[id]; done {
			continue
		}
		n, err := s.CountIncidents(id, since)
		if err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, nil
}
