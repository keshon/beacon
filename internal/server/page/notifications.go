package page

import (
	"net/http"
	"strings"
	"time"

	"github.com/keshon/beacon/internal/config"
	"github.com/keshon/beacon/internal/server/httpx"
	"github.com/keshon/beacon/internal/storage"

	"github.com/flosch/pongo2/v6"
)

// The notifications screen.
//
// Alerts were configuration only: four panels of fields inside Settings, and no
// way to learn whether any of it worked. A wrong token, a webhook that started
// returning 404, a policy quietly suppressing the second alert — all invisible.
// The question people actually have is not "what is configured" but "WILL I BE
// TOLD", and after a bad night, "why was I not".
//
// So the screen leads with what happened, not with what is set. Channels carry
// their real last result; the trail below shows every decision including the
// ones that chose not to send. Configuration lives on, but underneath — it is
// the answer to a question that is asked far less often.
type Notifications struct {
	Store  *storage.Store
	Cfg    *config.Live
	TplDir string
}

type channelCard struct {
	Name string
	// Configured is what the settings say.
	Configured bool
	Targets    int
	// The rest is what actually happened.
	Sent      int
	Failed    int
	Skipped   int
	LastAt    string
	LastError string
	Tone      string
	Verdict   string
}

type deliveryRow struct {
	At      string
	Monitor string
	Channel string
	Label   string
	Kind    string
	Status  string
	Reason  string
	Tone    string
}

func (h *Notifications) Serve(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	from := now.Add(-7 * 24 * time.Hour)
	cfg := h.Cfg.Load()

	health, _ := h.Store.GetChannelHealth(from, now)

	// "Configured" means enabled AND with somewhere to send: a channel switched
	// on with no targets is off in every way that matters.
	configured := map[string]int{}
	if cfg.Telegram.Enabled {
		configured["telegram"] = len(cfg.Telegram.Targets)
	}
	if cfg.Discord.Enabled {
		configured["discord"] = len(cfg.Discord.Webhooks)
	}
	if cfg.Email.Enabled {
		configured["email"] = len(cfg.Email.Targets)
	}
	if cfg.Webhook.Enabled {
		configured["webhook"] = len(cfg.Webhook.Webhooks)
	}

	var cards []channelCard
	for _, name := range []string{"telegram", "discord", "email", "webhook"} {
		hh := health[name]
		card := channelCard{
			Name:       name,
			Configured: configured[name] > 0,
			Targets:    configured[name],
			Sent:       hh.Sent, Failed: hh.Failed, Skipped: hh.Skipped,
			LastError: hh.LastError,
		}
		if !hh.LastAt.IsZero() {
			card.LastAt = hh.LastAt.Local().Format("02 Jan, 15:04")
		}
		card.Tone, card.Verdict = channelVerdict(card)
		cards = append(cards, card)
	}

	names := map[string]string{}
	if mons, err := h.Store.GetMonitors(); err == nil {
		for _, m := range mons {
			names[m.ID] = m.Name
		}
	}

	trail, _ := h.Store.GetDeliveries(from, now, 100)
	rows := make([]deliveryRow, 0, len(trail))
	for _, d := range trail {
		// The live name first — a monitor may have been renamed since — then
		// the one stored with the record, which is the only thing left once
		// the monitor is gone.
		name := names[d.MonitorID]
		if name == "" {
			name = d.MonitorName
		}
		if name == "" {
			name = "(deleted monitor)"
		}
		row := deliveryRow{
			At: d.At.Local().Format("02 Jan, 15:04:05"), Monitor: name,
			Channel: d.Channel, Label: d.Label, Kind: d.Kind,
			Status: d.Status, Reason: d.Reason,
		}
		switch d.Status {
		case "failed":
			row.Tone = "error"
		case "skipped":
			row.Tone = "warn"
		}
		rows = append(rows, row)
	}

	_ = httpx.Render(w, h.TplDir, "dashboard/notifications.html", pongo2.Context{
		"version":    buildVersion(),
		"nav_active": "notifications",
		"channels":   cards,
		"trail":      rows,
		"anyConfig":  configured["telegram"]+configured["discord"]+configured["email"]+configured["webhook"] > 0,
	})
}

// channelVerdict states what a channel has been doing, in a sentence.
//
// The case worth catching is "configured and never used": the settings say yes
// and reality has not been asked. A green tick there would be a lie of the most
// comfortable kind.
func channelVerdict(c channelCard) (tone, verdict string) {
	switch {
	case !c.Configured:
		return "", "not configured"
	case c.Failed > 0 && c.Sent == 0:
		return "error", "every attempt failed"
	case c.Failed > 0:
		return "warn", "some attempts failed"
	case c.Sent > 0:
		return "ok", "delivering"
	case c.Skipped > 0:
		return "", "nothing to send — alerts were suppressed"
	default:
		return "", "configured, never used"
	}
}

// channelTitle capitalises a channel name for display without a lookup table.
func channelTitle(name string) string {
	if name == "" {
		return name
	}
	return strings.ToUpper(name[:1]) + name[1:]
}
