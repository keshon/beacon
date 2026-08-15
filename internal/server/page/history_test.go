package page

import (
	"testing"
	"time"

	"github.com/keshon/beacon/internal/storage"
)

// A gap before the first check and a gap after it are different facts, and the
// strip used to draw them identically.
func TestHistorySkipsHoursBeforeTheFirstCheck(t *testing.T) {
	now := time.Date(2026, 8, 12, 10, 30, 0, 0, time.UTC)
	end := now.Truncate(time.Hour)

	// Checked for the last three hours and never before: a monitor added this
	// morning. Twenty-one bricks of "no data" would read as twenty-one bricks
	// of trouble.
	var rollups []storage.Rollup
	for i := 2; i >= 0; i-- {
		rollups = append(rollups, storage.Rollup{
			Hour: end.Add(-time.Duration(i) * time.Hour).Unix(), Total: 7,
		})
	}

	ticks := buildHistory(rollups, now)
	if len(ticks) != 3 {
		t.Fatalf("a monitor with three hours of history drew %d ticks, want 3", len(ticks))
	}
	for _, tk := range ticks {
		if tk.Gap {
			t.Fatal("an hour before the first check was drawn as a gap")
		}
	}
}

func TestHistoryPaintsGapsAfterCheckingStarted(t *testing.T) {
	now := time.Date(2026, 8, 12, 10, 30, 0, 0, time.UTC)
	end := now.Truncate(time.Hour)

	// Checked four hours ago, then silence. That silence is the thing the
	// strip exists to show.
	rollups := []storage.Rollup{
		{Hour: end.Add(-4 * time.Hour).Unix(), Total: 7},
	}

	ticks := buildHistory(rollups, now)
	if len(ticks) != 5 {
		t.Fatalf("drew %d ticks, want 5 (the checked hour and four since)", len(ticks))
	}
	for i, tk := range ticks[1:] {
		if !tk.Gap {
			t.Fatalf("hour %d after the last check is not a gap", i+1)
		}
		// A gap carries NO tone: the kit paints an untoned tick as track, and a
		// hole between coloured bricks reads as a hole without being announced.
		// Spelling it as a solid grey gave "nobody looked" the same weight as
		// "it broke", which is a louder claim than the fact deserves.
		if tk.Tone != "" {
			t.Fatalf("a gap must carry no tone, got %q", tk.Tone)
		}
	}
	// And the hour that WAS checked is green, so "up" and "nobody looked"
	// never share a colour.
	if ticks[0].Tone != "ok" {
		t.Fatalf("a checked hour with no failures must be ok, got %q", ticks[0].Tone)
	}
}
