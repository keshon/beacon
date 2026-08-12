package storage

import (
	"time"

	"github.com/keshon/datastore"
)

// Muting, and acknowledging.
//
// A monitor that is down during planned work still shouts every interval, and
// the only ways to stop it were to disable the monitor — which also stops the
// checking, so you lose the history you are about to want — or to endure it.
// Both are the same failure: the person has something to say and nowhere to
// say it.
//
// Muting silences the ALERTS and keeps the CHECKS. That distinction is the
// whole feature: the record of what happened during the maintenance window is
// exactly what gets asked about afterwards.
//
// An acknowledgement says "seen, and here is why" without silencing anything.
// It is addressed to the next person to look — including yourself at four in
// the morning, who will not remember.
type Mute struct {
	MonitorID string    `json:"monitor_id"`
	Until     time.Time `json:"until"`
	Note      string    `json:"note,omitempty"`
	At        time.Time `json:"at"`
}

func (m *Mute) Key() string { return m.MonitorID }

// Active reports whether the mute still holds.
func (m *Mute) Active(now time.Time) bool { return m != nil && now.Before(m.Until) }

// SetMute silences a monitor's alerts until a moment. A zero or past deadline
// lifts it: the same control both ways, because a mute you cannot cancel is a
// trap.
func (s *Store) SetMute(monitorID string, until time.Time, note string) error {
	if monitorID == "" {
		return nil
	}
	if until.IsZero() || !until.After(time.Now()) {
		return s.mutes.Delete(monitorID)
	}
	return s.mutes.Put(&Mute{MonitorID: monitorID, Until: until, Note: note, At: time.Now()})
}

// GetMute returns the monitor's mute, expired ones included: the caller asks
// Active when it needs the verdict, and the screen may want to say "was muted
// until 03:00" after it lapsed.
func (s *Store) GetMute(monitorID string) (*Mute, error) {
	m, ok := s.mutes.Get(monitorID)
	if !ok {
		return nil, nil
	}
	return m, nil
}

// GetMutes returns every mute still in force.
func (s *Store) GetMutes() (map[string]*Mute, error) {
	now := time.Now()
	out := map[string]*Mute{}
	for m := range s.mutes.All() {
		if m.Active(now) {
			out[m.MonitorID] = m
		}
	}
	return out, nil
}

// AcknowledgeIncident records that a person has seen the running incident and
// what they made of it.
//
// It changes nothing about the outage and everything about the next reader:
// "known, nightly backup" turns a red row into a decided one. If there is no
// running incident there is nothing to acknowledge, and saying so is better
// than storing a note nobody will find.
func (s *Store) AcknowledgeIncident(monitorID, by, note string) error {
	return s.db.Update(func(tx *datastore.Tx) error {
		open := s.openIncidentTx(tx, monitorID)
		if open == nil {
			return nil
		}
		open.AckBy = by
		open.AckNote = note
		open.AckAt = time.Now()
		return datastore.In(tx, s.incidents).Put(open)
	})
}
