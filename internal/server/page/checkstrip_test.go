package page

import (
	"strings"
	"testing"
	"time"

	"github.com/keshon/beacon/internal/storage"
)

func TestCheckStripGroupsByHourAndCountsTicks(t *testing.T) {
	now := time.Date(2026, 8, 12, 10, 30, 0, 0, time.UTC)
	end := now.Truncate(time.Hour)

	var recs []storage.CheckRecord
	// Two hours: one with a single check, one with seven. That difference is
	// the whole reason the strip exists — the hourly tick made them identical.
	recs = append(recs, storage.CheckRecord{Time: end.Add(-2 * time.Hour), Success: true})
	for i := 0; i < 7; i++ {
		recs = append(recs, storage.CheckRecord{
			Time:    end.Add(-time.Hour).Add(time.Duration(i) * 8 * time.Minute),
			Success: i != 3,
			Error:   "connection timeout",
		})
	}

	groups := buildCheckStrip(recs, 24, now)
	if len(groups) != 3 {
		t.Fatalf("got %d groups, want 3 (two with checks, one for the current hour)", len(groups))
	}
	if groups[0].N != 1 || len(groups[0].Ticks) != 1 {
		t.Fatalf("the quiet hour drew %d ticks, want 1", len(groups[0].Ticks))
	}
	if groups[1].N != 7 || len(groups[1].Ticks) != 7 {
		t.Fatalf("the busy hour drew %d ticks, want 7", len(groups[1].Ticks))
	}
	// The hour that failed names what failed: pointing at a two-pixel tick is
	// not possible, so the hour has to answer for it.
	if !strings.Contains(groups[1].Summary, "1 of 7 failed") ||
		!strings.Contains(groups[1].Summary, "connection timeout") {
		t.Fatalf("the failing hour does not say what went wrong: %q", groups[1].Summary)
	}
	if !groups[1].Wide {
		t.Fatal("a summary with a reason in it still asks for the narrow tooltip")
	}
}

func TestCheckStripSkipsHoursBeforeTheFirstCheck(t *testing.T) {
	now := time.Date(2026, 8, 12, 10, 30, 0, 0, time.UTC)
	end := now.Truncate(time.Hour)

	recs := []storage.CheckRecord{{Time: end, Success: true}}
	groups := buildCheckStrip(recs, 24, now)
	if len(groups) != 1 {
		t.Fatalf("a monitor added this hour drew %d groups, want 1", len(groups))
	}
	if groups[0].Empty {
		t.Fatal("the only hour with a check was drawn as a gap")
	}
}

func TestCheckStripPaintsNothingForAGapButKeepsThePlace(t *testing.T) {
	now := time.Date(2026, 8, 12, 10, 30, 0, 0, time.UTC)
	end := now.Truncate(time.Hour)

	// Checked three hours ago, then silence.
	recs := []storage.CheckRecord{{Time: end.Add(-3 * time.Hour), Success: true}}
	groups := buildCheckStrip(recs, 24, now)
	if len(groups) != 4 {
		t.Fatalf("got %d groups, want 4 (the checked hour and three since)", len(groups))
	}
	for i, g := range groups[1:] {
		if !g.Empty {
			t.Fatalf("hour %d after the last check is not a gap", i+1)
		}
		if g.N != 1 {
			t.Fatalf("an empty hour claims %d ticks, want 1", g.N)
		}
	}
}
