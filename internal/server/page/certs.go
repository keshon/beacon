package page

import (
	"sort"
	"strconv"
	"time"

	"github.com/keshon/beacon/internal/monitor"
)

// Certificate deadlines.
//
// This is the only kind of outage a monitor can warn about BEFORE it happens:
// the date is known, it does not depend on load or luck, and the fix takes
// minutes if you start in time and hours if you start after.
//
// The screen says it once, in one line, and only when it is close. A row per
// certificate would be a list of dates nobody reads; a single sentence about
// the nearest one is a thing a person acts on.

// certWarnWithin is how early the warning appears. Long enough to renew
// without hurrying, short enough that the line is not permanent furniture —
// a warning that is always on screen stops being a warning.
const certWarnWithin = 21 * 24 * time.Hour

type certNote struct {
	// Text is the whole note, already worded.
	Text string
	// Tone is empty while there is time and "warn" when there is not much.
	Tone string
	// MonitorID is the nearest deadline's monitor, for the link.
	MonitorID string
}

type certDeadline struct {
	Name string
	ID   string
	At   time.Time
}

// buildCertNote words the certificate line, or returns nil when there is
// nothing worth saying.
func buildCertNote(states map[string]*monitor.MonitorState, names map[string]string, now time.Time) *certNote {
	var deadlines []certDeadline
	for id, st := range states {
		if st == nil || st.CertExpiry.IsZero() {
			continue
		}
		name := names[id]
		if name == "" {
			continue // peer or deleted monitor: nothing to link to
		}
		deadlines = append(deadlines, certDeadline{Name: name, ID: id, At: st.CertExpiry})
	}
	if len(deadlines) == 0 {
		return nil
	}
	sort.Slice(deadlines, func(i, j int) bool { return deadlines[i].At.Before(deadlines[j].At) })

	first := deadlines[0]
	left := first.At.Sub(now)
	if left > certWarnWithin {
		return nil
	}

	note := &certNote{MonitorID: first.ID, Tone: "warn"}
	switch {
	case left <= 0:
		note.Text = "Certificate for " + first.Name + " has expired"
	case left < 24*time.Hour:
		note.Text = "Certificate for " + first.Name + " expires within a day"
	default:
		days := int(left.Hours() / 24)
		note.Text = "Certificate for " + first.Name + " expires in " + strconv.Itoa(days) + " day"
		if days != 1 {
			note.Text += "s"
		}
	}

	// The rest are counted, not listed: the nearest deadline is the one that
	// needs doing, and "two more" is enough to know it is not an isolated case.
	if rest := len(deadlines) - 1; rest > 0 {
		note.Text += ". " + strconv.Itoa(rest) + " more tracked"
	}
	return note
}

// certLine describes one monitor's certificate for its own screen, where the
// exact date belongs.
func certLine(st *monitor.MonitorState, now time.Time) (text, tone string) {
	if st == nil || st.CertExpiry.IsZero() {
		return "", ""
	}
	left := st.CertExpiry.Sub(now)
	date := st.CertExpiry.Local().Format("2 January 2006")
	switch {
	case left <= 0:
		return "expired " + date, "error"
	case left <= certWarnWithin:
		return humanDuration(left) + " left, until " + date, "warn"
	default:
		return "until " + date, ""
	}
}
