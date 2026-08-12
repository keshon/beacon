package page

import (
	"fmt"
	"time"

	"github.com/keshon/beacon/internal/storage"
)

// The history strip on a row.
//
// It used to be drawn by the browser from raw samples: one tick per check, as
// many as fitted. That made the strip lie about time — for a monitor checked
// every thirty seconds, twenty-four ticks were twenty-four MINUTES while the
// label said the last day. Two rows side by side covered different spans, so
// comparing them by eye compared nothing.
//
// Now the server builds it from hourly buckets: one tick is one hour, the same
// hour on every row. What the strip shows and what it claims to show agree.

// HistTick is one hour of one monitor.
type HistTick struct {
	// Tone is the kit's tone: empty means an hour without remarks.
	Tone string
	// Gap marks an hour that had no checks although this monitor was already
	// being checked. Nobody looked — which is a fact about US, not about the
	// target, so the tick keeps its place and carries no mark. The alarm, when
	// it is due, is the badge on the row: "no check since 07:14".
	Gap   bool
	Title string
}

// historyHours is how many hours a row's strip covers.
const historyHours = 24

// buildHistory turns hourly buckets into a fixed-width strip ending at now.
//
// The width is fixed on purpose: every row covers the same hours, so a glance
// down the column compares like with like. Missing hours become gaps rather
// than being squeezed out.
func buildHistory(rollups []storage.Rollup, now time.Time) []HistTick {
	byHour := make(map[int64]storage.Rollup, len(rollups))
	for _, r := range rollups {
		byHour[r.Hour] = r
	}

	end := now.UTC().Truncate(time.Hour)
	out := make([]HistTick, 0, historyHours)
	// Hours BEFORE the first check ever are not drawn at all.
	//
	// A monitor added an hour ago used to render twenty-three empty bricks —
	// a wall of "no data" that looks like a wall of trouble, and reads as the
	// same thing as a monitor that stopped being checked this morning. They
	// are not the same thing: one has no history, the other lost it. A short
	// strip says "this is all we have" without a single mark of alarm.
	started := false
	for i := historyHours - 1; i >= 0; i-- {
		hour := end.Add(-time.Duration(i) * time.Hour)
		label := hour.Local().Format("15:04")

		r, ok := byHour[hour.Unix()]
		if !ok || r.Total == 0 {
			if !started {
				continue
			}
			out = append(out, HistTick{Gap: true, Title: label + " — no checks"})
			continue
		}
		started = true
		tick := HistTick{Title: fmt.Sprintf("%s — %s", label, plural(r.Total, "check", "checks"))}
		if r.Failed > 0 {
			tick.Tone = "error"
			tick.Title = fmt.Sprintf("%s — %d of %d failed", label, r.Failed, r.Total)
		}
		out = append(out, tick)
	}
	return out
}

// historyLabel is the strip's accessible name: the picture in one sentence,
// because a row of twenty-four spans reads as nothing to a screen reader.
func historyLabel(ticks []HistTick) string {
	bad, gaps := 0, 0
	for _, t := range ticks {
		switch {
		case t.Gap:
			gaps++
		case t.Tone != "":
			bad++
		}
	}
	switch {
	case bad == 0 && gaps == 0:
		return "Last 24 hours: no failures"
	case bad == 0:
		return fmt.Sprintf("Last 24 hours: no failures, %d hours without checks", gaps)
	case gaps == 0:
		return fmt.Sprintf("Last 24 hours: %d hours with failures", bad)
	}
	return fmt.Sprintf("Last 24 hours: %d hours with failures, %d without checks", bad, gaps)
}
