package storage_test

import (
	"testing"
	"time"

	"github.com/keshon/beacon/internal/storage"
)

func deliver(t *testing.T, st *storage.Store, at time.Time, channel, status, reason string) {
	t.Helper()
	if err := st.RecordDelivery(storage.Delivery{
		At: at, MonitorID: "m", Channel: channel, Label: "chat 42",
		Kind: "down", Status: status, Reason: reason,
	}); err != nil {
		t.Fatal(err)
	}
}

// The trail must keep suppressions, not just sends: they are the answer to
// "why was I not told".
func TestDeliveriesKeepSkips(t *testing.T) {
	st := newStore(t)
	now := time.Now()

	deliver(t, st, now.Add(-3*time.Minute), "telegram", "sent", "")
	deliver(t, st, now.Add(-2*time.Minute), "telegram", "skipped", "policy alerts once and it already did")
	deliver(t, st, now.Add(-time.Minute), "email", "failed", "550 mailbox unavailable")

	list, err := st.GetDeliveries(now.Add(-time.Hour), now, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("want three records, got %d", len(list))
	}
	if list[0].Status != "failed" {
		t.Fatalf("newest first is broken: %#v", list[0])
	}
	var skipped bool
	for _, d := range list {
		if d.Status == "skipped" && d.Reason != "" {
			skipped = true
		}
	}
	if !skipped {
		t.Fatal("the suppression lost its reason")
	}
}

// The health summary is what the channel panel stands on: counts plus the last
// thing that went wrong.
func TestChannelHealthSummarises(t *testing.T) {
	st := newStore(t)
	now := time.Now()

	deliver(t, st, now.Add(-5*time.Minute), "telegram", "sent", "")
	deliver(t, st, now.Add(-4*time.Minute), "telegram", "sent", "")
	deliver(t, st, now.Add(-3*time.Minute), "telegram", "skipped", "duplicate")
	deliver(t, st, now.Add(-2*time.Minute), "email", "failed", "550 mailbox unavailable")

	health, err := st.GetChannelHealth(now.Add(-time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	tg := health["telegram"]
	if tg.Sent != 2 || tg.Skipped != 1 || tg.Failed != 0 {
		t.Fatalf("telegram: %#v", tg)
	}
	if tg.LastStatus != "skipped" || tg.LastError != "" {
		t.Fatalf("telegram last: %#v", tg)
	}
	em := health["email"]
	if em.Failed != 1 || em.LastError != "550 mailbox unavailable" {
		t.Fatalf("email should carry its last error: %#v", em)
	}
}

// The trail is bounded, and a monitor flapping through four channels writes
// fast. Oldest goes first.
func TestDeliveriesAreBounded(t *testing.T) {
	st := newStore(t)
	base := time.Now().Add(-time.Hour)

	for i := 0; i < 2100; i++ {
		deliver(t, st, base.Add(time.Duration(i)*time.Second), "telegram", "sent", "")
	}
	list, err := st.GetDeliveries(base.Add(-time.Hour), time.Now(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2000 {
		t.Fatalf("trail: want 2000, got %d", len(list))
	}
	// The survivors are the newest ones.
	oldest := list[len(list)-1].At
	if oldest.Before(base.Add(99 * time.Second)) {
		t.Fatalf("the wrong end was trimmed: oldest kept is %v", oldest)
	}
}

// Records older than the horizon go even when the count is nowhere near.
func TestDeliveriesAgeOut(t *testing.T) {
	st := newStore(t)
	now := time.Now()

	deliver(t, st, now.Add(-60*24*time.Hour), "telegram", "sent", "")
	deliver(t, st, now, "telegram", "sent", "")

	list, err := st.GetDeliveries(now.Add(-365*24*time.Hour), now, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("want only the fresh record, got %d", len(list))
	}
}
