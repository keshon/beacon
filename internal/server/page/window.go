package page

import (
	"strconv"
	"time"

	"github.com/keshon/beacon/internal/storage"
)

// The history window.
//
// The strip always has the SAME number of ticks; what changes with the window
// is how much time one tick holds. A day gives an hour per tick, a week seven
// hours, a month thirty. Rows stay comparable at any window because the column
// keeps its shape, and no window ever produces a strip of seven marks or of
// seven hundred.
type histWindow struct {
	Key   string
	Label string
	Span  time.Duration
	// PerTick is how much time one tick covers.
	PerTick time.Duration
}

var histWindows = []histWindow{
	{Key: "day", Label: "24 hours", Span: 24 * time.Hour, PerTick: time.Hour},
	{Key: "week", Label: "7 days", Span: 7 * 24 * time.Hour, PerTick: 7 * time.Hour},
	{Key: "month", Label: "30 days", Span: 30 * 24 * time.Hour, PerTick: 30 * time.Hour},
}

func windowByKey(key string) histWindow {
	for _, w := range histWindows {
		if w.Key == key {
			return w
		}
	}
	return histWindows[0]
}

// tickCount is fixed on purpose — see the note above.
const tickCount = 24

// buildWindowHistory folds hourly buckets into a fixed-width strip for a window.
func buildWindowHistory(rollups []storage.Rollup, w histWindow, now time.Time) []HistTick {
	byHour := make(map[int64]storage.Rollup, len(rollups))
	for _, r := range rollups {
		byHour[r.Hour] = r
	}

	end := now.UTC().Truncate(time.Hour).Add(time.Hour)
	out := make([]HistTick, 0, tickCount)
	// Same rule as the hourly strip: nothing is drawn for the time before the
	// first check ever, and a gap AFTER it is painted. See buildHistory.
	started := false
	for i := tickCount - 1; i >= 0; i-- {
		to := end.Add(-time.Duration(i) * w.PerTick)
		from := to.Add(-w.PerTick)

		total, failed := 0, 0
		for h := from; h.Before(to); h = h.Add(time.Hour) {
			if r, ok := byHour[h.Unix()]; ok {
				total += r.Total
				failed += r.Failed
			}
		}

		label := tickLabel(from, w)
		if total == 0 {
			if !started {
				continue
			}
			out = append(out, HistTick{Gap: true, Title: label + " — no checks"})
			continue
		}
		started = true
		// Same three tones as the hourly strip, for the same reason: a slice
		// that was up must not look like a slice nobody checked. See HistTick.
		tick := HistTick{
			Tone:  "ok",
			Title: label + " — " + strconv.Itoa(total) + " checks, all up",
		}
		if failed > 0 {
			tick.Tone = "error"
			tick.Title = label + " — " + strconv.Itoa(failed) + " of " + strconv.Itoa(total) + " failed"
		}
		out = append(out, tick)
	}
	return out
}

// tickLabel names the slice of time a tick stands for, at the resolution the
// window makes meaningful: an hour needs the clock, a month needs the date.
func tickLabel(from time.Time, w histWindow) string {
	local := from.Local()
	if w.PerTick < 24*time.Hour {
		return local.Format("02 Jan 15:04")
	}
	return local.Format("02 Jan")
}
