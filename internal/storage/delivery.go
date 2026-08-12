package storage

import (
	"sort"
	"strconv"
	"time"

	"github.com/keshon/datastore"
)

// Notification deliveries.
//
// Alerts used to be write-only: you configured a channel and hoped. A wrong
// token, a webhook that started returning 404, a policy that suppressed the
// second alert — all of it was invisible, and the only place any of it showed
// up was a log line nobody reads.
//
// Every decision is recorded now, including the ones that chose NOT to send.
// That is the half that matters: after an outage nobody heard about, the
// question is "why was I not told", and "the policy alerts once and it already
// had" is an answer where silence is not.
//
// Nothing secret is kept. Label is the display-safe name built in the notify
// package — a host, a chat id, an address — never the receiver key, whose tail
// is the webhook token.
type Delivery struct {
	At        time.Time `json:"at"`
	MonitorID string    `json:"monitor_id"`
	// MonitorName is stored WITH the record, not looked up when it is read.
	//
	// The trail outlives the monitor: delete one and its history turned into
	// rows saying "(deleted monitor) down", which is the moment a journal
	// stops being a journal — it forgets what it is about exactly when someone
	// needs it. A name costs a few bytes; the lookup cost the meaning.
	MonitorName string `json:"monitor_name,omitempty"`
	Channel     string `json:"channel"`
	Label       string `json:"label"`
	// Kind is "down" or "recovered".
	Kind string `json:"kind"`
	// Status is sent · failed · skipped.
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

func (d *Delivery) Key() string {
	return strconv.FormatInt(d.At.UnixNano(), 10) + "/" + d.Channel + "/" + d.Label
}

const (
	// deliveryMaxAge is how long the trail is kept. Long enough to answer
	// "did the alert go out last week", short enough to stay small.
	deliveryMaxAge = 30 * 24 * time.Hour
	// deliveryLimit caps the trail regardless of age: a monitor flapping every
	// thirty seconds through four channels writes fast.
	deliveryLimit = 2000
)

// RecordDelivery stores one decision and trims the trail.
func (s *Store) RecordDelivery(d Delivery) error {
	if d.At.IsZero() {
		d.At = time.Now()
	}
	return s.db.Update(func(tx *datastore.Tx) error {
		// Trim BEFORE the put, to one below the cap. Trimming after would have
		// to know whether the sorted index already counts a record staged in
		// this same transaction — it does not — and the trail would settle one
		// over the limit forever. Making room first needs no such assumption.
		if err := s.pruneDeliveriesTx(tx, d.At); err != nil {
			return err
		}
		cp := d
		return datastore.In(tx, s.deliveries).Put(&cp)
	})
}

func (s *Store) pruneDeliveriesTx(tx *datastore.Tx, now time.Time) error {
	idx := datastore.InSorted(tx, s.deliveriesByTime)
	col := datastore.In(tx, s.deliveries)

	for _, old := range idx.Range(0, now.Add(-deliveryMaxAge).UnixNano()) {
		if err := col.Delete(old.Key()); err != nil {
			return err
		}
	}
	if extra := idx.Len() - (deliveryLimit - 1); extra > 0 {
		for _, old := range idx.Asc(extra, 0) {
			if err := col.Delete(old.Key()); err != nil {
				return err
			}
		}
	}
	return nil
}

// GetDeliveries returns the trail within [from, to], newest first.
func (s *Store) GetDeliveries(from, to time.Time, limit int) ([]Delivery, error) {
	found := s.deliveriesByTime.Range(from.UnixNano(), to.UnixNano())
	sort.Slice(found, func(i, j int) bool { return found[i].At.After(found[j].At) })
	if limit > 0 && limit < len(found) {
		found = found[:limit]
	}
	out := make([]Delivery, len(found))
	for i, d := range found {
		out[i] = *d
	}
	return out, nil
}

// ChannelHealth is what a channel has been doing lately: the counts, and the
// last thing that went wrong.
type ChannelHealth struct {
	Channel    string
	Sent       int
	Failed     int
	Skipped    int
	LastAt     time.Time
	LastStatus string
	LastError  string
}

// GetChannelHealth summarises the trail per channel over a window.
//
// A channel that is "enabled" and has never delivered anything is the case
// worth catching: configuration says yes, reality has not been asked.
func (s *Store) GetChannelHealth(from, to time.Time) (map[string]ChannelHealth, error) {
	out := map[string]ChannelHealth{}
	for _, d := range s.deliveriesByTime.Range(from.UnixNano(), to.UnixNano()) {
		h := out[d.Channel]
		h.Channel = d.Channel
		switch d.Status {
		case "sent":
			h.Sent++
		case "failed":
			h.Failed++
		case "skipped":
			h.Skipped++
		}
		if d.At.After(h.LastAt) {
			h.LastAt = d.At
			h.LastStatus = d.Status
			h.LastError = ""
			if d.Status == "failed" {
				h.LastError = d.Reason
			}
		}
		out[d.Channel] = h
	}
	return out, nil
}
