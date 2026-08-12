package page

import (
	"net/http"
	"sort"

	"github.com/keshon/beacon/internal/cluster"
	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/server/httpx"
	"github.com/keshon/beacon/internal/storage"

	"github.com/flosch/pongo2/v6"
)

// The peers screen.
//
// Peers used to be a section at the top of the monitors list, which is the
// wrong place twice over. They are almost never what the list is opened for,
// and they pushed the monitors down every single time. And when a peer DOES
// need attention — a sync that has been failing for a day — one dim line under
// a node id said far too little.
//
// The screen answers what a distributed setup is actually asked: who is alive,
// when we last heard from them, what they see, and what is wrong with the sync.
type Peers struct {
	Store   *storage.Store
	Cfg     *config.Live
	Cluster *cluster.Runtime
	TplDir  string
}

type peerRow struct {
	cluster.NetworkNode
	// Self marks this node. It is in the list because a node that cannot see
	// itself is a symptom, and because "16 monitors here" is the baseline the
	// other rows are read against.
	Self bool
	Tone string
}

func (h *Peers) Serve(w http.ResponseWriter, r *http.Request) {
	enabled := h.Cfg.Load().Network.Enabled

	var rows []peerRow
	live, dead, warnings := 0, 0, 0

	if enabled && h.Cluster != nil {
		state, _ := h.Store.GetAllState()
		monitors, _ := h.Store.GetMonitors()
		view, err := h.Cluster.DashboardView(state, monitors)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, n := range view.NetworkNodes {
			row := peerRow{NetworkNode: n, Self: n.Status == "self"}
			switch n.Status {
			case "live":
				row.Tone = "ok"
				live++
			case "dead":
				row.Tone = "error"
				dead++
			case "self":
				row.Tone = "running"
				// This node is alive by construction — it is rendering the page.
				// Leaving it out of the count made the badge read "1 live" of
				// two healthy nodes, which is the screen telling the reader
				// half the cluster is down while nothing is wrong.
				live++
			default:
				row.Tone = "neutral"
			}
			warnings += len(n.SyncWarnings)
			rows = append(rows, row)
		}
		// This node first — it is the reference — then the rest by address, so
		// the list does not reorder itself as peers come and go.
		sort.SliceStable(rows, func(i, j int) bool {
			if rows[i].Self != rows[j].Self {
				return rows[i].Self
			}
			return rows[i].URL < rows[j].URL
		})
	}

	_ = httpx.Render(w, h.TplDir, "dashboard/peers.html", pongo2.Context{
		"version":    buildVersion(),
		"nav_active": "peers",
		"enabled":    enabled,
		"peers":      rows,
		"live":       live,
		"dead":       dead,
		"warnings":   warnings,
	})
}
