package page

import (
	"strconv"
	"time"

	"github.com/keshon/beacon/internal/monitor"
)

// The last check, phase by phase.
//
// A check is a chain, and until now the screen showed only its total. The chain
// is where the answer lives: DNS resolved, the socket opened, the handshake
// finished, and then the server said nothing for ten seconds — that is a
// different problem, with a different owner, from a name that would not resolve.
//
// Phases that were not measured are not shown. A reused connection skips DNS,
// TCP and TLS; plain HTTP has no handshake. A zero there is a fact about the
// connection, not a gap in the data, and inventing a "0 ms" row would suggest
// the step happened instantly when it did not happen at all.

type stepRow struct {
	Name  string
	Sub   string
	Meta  string
	State string // ok · warn · failed
}

// buildSteps turns the last check's phases into the kit's step rows.
func buildSteps(st *monitor.MonitorState, target string, now time.Time) []stepRow {
	if st == nil || !st.Phases.Any() {
		return nil
	}
	p := st.Phases
	failed := st.Status == monitor.StatusDown

	var out []stepRow
	add := func(name, sub string, d time.Duration, measured bool) {
		if !measured {
			return
		}
		out = append(out, stepRow{
			Name:  name,
			Sub:   sub,
			Meta:  ms(d),
			State: "ok",
		})
	}

	add("DNS", "name resolved", p.DNS, p.DNS > 0)
	add("TCP", "socket opened", p.TCP, p.TCP > 0)
	add("TLS", "handshake finished", p.TLS, p.TLS > 0)

	// The last step carries the verdict: everything before it demonstrably
	// worked, or the check would not have got this far.
	switch {
	case p.Server > 0 && !failed:
		out = append(out, stepRow{
			Name: "HTTP", Sub: "response received", Meta: ms(p.Server), State: "ok",
		})
	case p.Server > 0 && failed:
		out = append(out, stepRow{
			Name: "HTTP", Sub: "answered, but the check did not pass",
			Meta: ms(p.Server), State: "failed",
		})
	case failed:
		out = append(out, stepRow{
			Name: "HTTP", Sub: "no response",
			Meta: ms(st.Latency) + " · timeout", State: "failed",
		})
	}

	// Certificate warnings belong to the handshake that saw them.
	if text, tone := certLine(st, now); tone != "" {
		for i := range out {
			if out[i].Name == "TLS" {
				out[i].Sub = "certificate " + text
				if out[i].State == "ok" {
					out[i].State = tone
				}
			}
		}
	}
	return out
}

// ms prints a duration the way a latency is read: whole milliseconds up to a
// second, then seconds with one decimal.
func ms(d time.Duration) string {
	if d <= 0 {
		return "—"
	}
	if d < time.Second {
		return strconv.FormatInt(d.Milliseconds(), 10) + "ms"
	}
	return strconv.FormatFloat(d.Seconds(), 'f', 1, 64) + "s"
}
