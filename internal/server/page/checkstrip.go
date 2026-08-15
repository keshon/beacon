package page

import (
	"fmt"
	"strconv"
	"time"

	"github.com/keshon/beacon/internal/storage"
)

// The per-check strip, grouped by hour.
//
// The row in the list draws one tick per HOUR, and it has to: twenty rows must
// line up so that a glance down the column compares the same hours. On the
// monitor's own screen there is one strip and no column to line up with, and
// the hourly tick throws away the thing that screen is for — an hour holding
// one check looked exactly like an hour holding a hundred.
//
// Here a tick is a CHECK and a group is an hour. The group's width is its
// share of the checks, so a dense monitor looks dense.
//
// The tooltip hangs on the GROUP, not on the tick, and that is a consequence
// of the same arithmetic: a tick is about two pixels wide on a narrow layout,
// and a two-pixel hover target is not one. The hour is thirty pixels and can
// be pointed at, so it carries the detail — including WHAT failed, not only
// how many did.
type CheckTick struct {
	// Tone is the kit's tone: "ok" for a check that passed, "error" for one
	// that did not. Never empty — a check always has an outcome, and grey is
	// reserved for the hour nobody checked (see HistTick).
	Tone string
}

// CheckGroup is one hour.
type CheckGroup struct {
	// N is the number of ticks. It becomes --n in the markup, and the kit
	// makes the group's width proportional to it.
	N     int
	Label string
	// Axis is the hour written under the strip. AxisMajor marks the hours
	// that keep their label when the strip is too narrow for all of them.
	Axis      string
	AxisMajor bool
	// Summary is what the hour says as a whole, for its own tooltip.
	Summary string
	Ticks   []CheckTick
	// Wide asks for the roomier tooltip: a sentence, not a label.
	Wide bool
	// Empty marks an hour that had no checks although checking had started.
	Empty bool
}

// buildCheckStrip groups raw check records into hourly groups over the window
// ending at now.
//
// Hours before the first check are skipped, exactly as in the hourly strip:
// a monitor added an hour ago should not open with a wall of nothing.
func buildCheckStrip(records []storage.CheckRecord, hours int, now time.Time) []CheckGroup {
	byHour := make(map[int64][]storage.CheckRecord, hours)
	for _, r := range records {
		h := r.Time.UTC().Truncate(time.Hour).Unix()
		byHour[h] = append(byHour[h], r)
	}

	end := now.UTC().Truncate(time.Hour)
	out := make([]CheckGroup, 0, hours)
	started := false
	for i := hours - 1; i >= 0; i-- {
		hour := end.Add(-time.Duration(i) * time.Hour)
		label := hour.Local().Format("15:04")

		recs := byHour[hour.Unix()]
		if len(recs) == 0 {
			if !started {
				continue
			}
			out = append(out, CheckGroup{
				N: 1, Label: label, Axis: axisLabel(hour), AxisMajor: axisMajor(hour), Empty: true,
				Summary: label + " — no checks",
			})
			continue
		}
		started = true

		g := CheckGroup{
			N: len(recs), Label: label,
			Axis: axisLabel(hour), AxisMajor: axisMajor(hour),
			Ticks: make([]CheckTick, 0, len(recs)),
		}
		failed := 0
		for _, r := range recs {
			t := CheckTick{Tone: "ok"}
			if !r.Success {
				failed++
				t.Tone = "error"
			}
			g.Ticks = append(g.Ticks, t)
		}
		g.Summary = fmt.Sprintf("%s — %s", label, plural(len(recs), "check", "checks"))
		if failed > 0 {
			// The hour that failed says WHAT failed, not just how many.
			//
			// A tick is two pixels wide at a narrow layout, so hovering an
			// individual check is not a thing a person can do. The group is
			// thirty pixels and hoverable — so the detail belongs on the
			// group, and pointing at an hour has to be enough to learn what
			// went wrong in it.
			g.Summary = fmt.Sprintf("%s — %d of %d failed · %s", label, failed, len(recs), firstFailure(recs))
			g.Wide = true
		}
		out = append(out, g)
	}
	return out
}

// checkStripLabel is the strip's accessible name. A row of two hundred spans
// reads as nothing to a screen reader, so the picture goes into one sentence.
func checkStripLabel(groups []CheckGroup, hours int) string {
	checks, failed, gaps := 0, 0, 0
	for _, g := range groups {
		if g.Empty {
			gaps++
			continue
		}
		checks += len(g.Ticks)
		for _, t := range g.Ticks {
			if t.Tone == "error" {
				failed++
			}
		}
	}
	head := fmt.Sprintf("Last %d hours: %s", hours, plural(checks, "check", "checks"))
	switch {
	case checks == 0:
		return fmt.Sprintf("Last %d hours: no checks", hours)
	case failed > 0 && gaps > 0:
		return fmt.Sprintf("%s, %d failed, %d hours without checks", head, failed, gaps)
	case failed > 0:
		return fmt.Sprintf("%s, %d failed", head, failed)
	case gaps > 0:
		return fmt.Sprintf("%s, none failed, %d hours without checks", head, gaps)
	default:
		return head + ", none failed"
	}
}

// axisMajorEvery marks the hours that survive on a narrow strip.
//
// EVERY hour is labelled — the axis exists so nobody has to hover a brick to
// learn it is 13:00. But 24 labels need width: a bare hour number is about
// 16px, so all of them want ~480px and only the major ones want ~160px. Which
// set is shown is decided by the STRIP, in a container query, because the
// server cannot know how wide the reader's window is. The server's job is to
// say which hours are major; three divides 24 evenly, so the row keeps its
// first and last label either way.
//
// The exact minute stays in the tooltip. The axis answers "when", the tooltip
// answers "what happened".
const axisMajorEvery = 3

// axisLabel returns the hour to print under a group. It reads the LOCAL hour,
// because the axis is read by a person in their own timezone, not by the store
// in UTC.
func axisLabel(hour time.Time) string {
	return fmt.Sprintf("%02d", hour.Local().Hour())
}

// axisMajor reports whether the hour keeps its label on a narrow strip.
func axisMajor(hour time.Time) bool {
	return hour.Local().Hour()%axisMajorEvery == 0
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}

// firstFailure names the first failed check in the hour: its time and what it
// came back with. The FIRST, not the last — what broke is more useful than
// what it looked like on the way out, the same rule the incident record uses.
func firstFailure(recs []storage.CheckRecord) string {
	for _, r := range recs {
		if r.Success {
			continue
		}
		what := r.Error
		if what == "" {
			what = "failed"
		}
		if len(what) > 60 {
			what = what[:57] + "…"
		}
		return r.Time.Local().Format("15:04:05") + " " + what
	}
	return ""
}
