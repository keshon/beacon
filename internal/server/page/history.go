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
//
// Three outcomes, three tones, and the third one is the absence of a tone:
//
//	ok      the hour was checked and nothing failed
//	error   the hour had at least one failure
//	(none)  nobody checked — the kit paints an untoned tick as --track
//
// The third one stays UNSPOKEN on purpose. A gap between coloured bricks is
// already legible as a gap: the eye finds a hole in a dense row without being
// told, and reads it as absence rather than as a verdict. Painting it a solid
// grey gave "nobody looked" the same visual weight as "it broke", which is a
// louder claim than the fact deserves.
//
// The kit's own strip leaves "up" untoned on the argument that a wall of green
// stops being a signal. That argument is about strips where most of the width
// is routine and the exception is what you came for. An uptime monitor is the
// other case: "it was up" IS the answer, and while it was untoned a healthy
// hour and an unchecked one were the same grey brick — the one confusion this
// screen must not have. Green resolves it, and the gap keeps the track it
// always had.
type HistTick struct {
	// Tone is the kit's tone: "ok", "error", or empty for an hour nobody
	// checked — the kit paints an untoned tick as track.
	Tone string
	// Gap marks an hour that had no checks although this monitor was already
	// being checked. Nobody looked — a fact about US, not about the target.
	Gap bool
	// Axis is the hour written under the strip. AxisMajor marks the hours
	// that keep their label when the strip is too narrow for all of them.
	Axis      string
	AxisMajor bool
	Title     string
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
			out = append(out, HistTick{
				Gap: true, Axis: axisLabel(hour), AxisMajor: axisMajor(hour),
				Title: label + " — no checks",
			})
			continue
		}
		started = true
		tick := HistTick{
			Tone:      "ok",
			Axis:      axisLabel(hour),
			AxisMajor: axisMajor(hour),
			Title:     fmt.Sprintf("%s — %s, all up", label, plural(r.Total, "check", "checks")),
		}
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
		case t.Tone == "error":
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
