package alerts

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/mail"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// Draft is a discovery's draft the agent has checked: what an email says
// about it.
type Draft struct {
	Tenant  store.Tenant
	App     store.App
	Version int
	Lines   []string // what changes, one line each
	Measure string   // the agent's check, in words
}

// DraftReady emails a tenant's administrators and extra addresses that a
// discovery's draft is checked and waits to be published (alert kind
// draft_ready). The draft itself is never published by Equalizer.
func DraftReady(ctx context.Context, st Store, m mail.Sender, panelURL string, d Draft, now time.Time, log *slog.Logger) {
	settings, err := st.Alerts(ctx, d.Tenant.ID)
	if err != nil || !settings.Enabled || !settings.Wants("draft_ready") {
		return
	}
	to := recipients(ctx, st, d.Tenant.ID, settings)
	if len(to) == 0 {
		return
	}
	link := fmt.Sprintf("%s/t/%s/sites/%s/maps/%d", strings.TrimSuffix(panelURL, "/"), d.Tenant.ID, d.App.ID, d.Version)
	emails := make([]string, 0, len(to))
	for _, rc := range to {
		msg := composeDraft(d, link, now.In(rc.zone))
		msg.To = rc.email
		if err := m.Send(ctx, msg); err != nil {
			log.ErrorContext(ctx, "draft email not sent", slog.String("error", err.Error()))
			continue
		}
		emails = append(emails, rc.email)
	}
	if len(emails) == 0 {
		return
	}
	tenant := d.Tenant.ID
	_ = st.Record(ctx, nil, &tenant, "alert.sent", d.App.ID, map[string]any{
		"changes": 1, "recipients": len(emails), "to": emails,
		"alerts": []map[string]any{{"site": d.App.Name, "check": "Draft ready", "down": false,
			"detail": fmt.Sprintf("version %d", d.Version)}},
	})
}

func composeDraft(d Draft, link string, at time.Time) mail.Message {
	var b strings.Builder
	for _, l := range d.Lines {
		fmt.Fprintf(&b, "- %s\n", l)
	}
	if b.Len() == 0 {
		b.WriteString("- changes to the map (see the draft)\n")
	}
	body := fmt.Sprintf("Hello,\n\nthe agent studied %s (%s) and prepared version %d of its map (%s).\n\n"+
		"What changes:\n%s\nThe agent's check: %s\n\nReview it and publish it: %s\n"+
		"Nothing is published until an administrator does.\n\n"+
		"You receive this as an administrator of %s (Settings → Alerts, \"draft ready\").\n",
		d.App.Name, d.App.PublicDomain, d.Version, When(at), b.String(), d.Measure, link, d.Tenant.Name)
	return mail.Message{Subject: fmt.Sprintf("Equalizer: a new map for %s is ready to publish", oneLine(d.App.Name)), Body: body}
}

// oneLine keeps a name on one line: a header with a line break is refused.
func oneLine(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, s)
}
