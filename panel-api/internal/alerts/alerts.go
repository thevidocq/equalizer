// Package alerts emails a tenant when one of its sites stops working and
// when it works again: connector offline, legacy site failing, site
// reachable again on its old IP. A problem must last a few minutes before
// an email; each change is sent once, even across restarts.
package alerts

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/health"
	"github.com/thevidocq/equalizer/panel-api/internal/mail"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// Watched are the checks that send alerts; a missing map is a site being
// set up, not an incident.
var Watched = map[string]bool{"Connector": true, "Legacy site": true, "Isolation": true, "Certificate": true}

// Reminders before a certificate ends, in days: one email each.
var Reminders = []int{30, 14, 7, 1}

const expiryCheck = "Certificate expiry"

// Store is what the runner needs from the database.
type Store interface {
	Tenants(ctx context.Context, only *string) ([]store.Tenant, error)
	Users(ctx context.Context, tenant string) ([]auth.User, error)
	Alerts(ctx context.Context, tenant string) (store.AlertSettings, error)
	SiteAlerts(ctx context.Context, app string) (map[string]store.SiteAlert, error)
	SaveSiteAlert(ctx context.Context, app, check string, a store.SiteAlert) error
	Record(ctx context.Context, userID *int64, tenant *string, action, target string, detail map[string]any) error
}

// Statuses evaluates a tenant's sites (the status page).
type Statuses func(ctx context.Context, tenant string) ([]health.Site, bool, error)

// Runner checks every tenant once per Every.
type Runner struct {
	Store    Store
	Statuses Statuses
	Mail     mail.Sender
	PanelURL string
	Every    time.Duration // default 2 minutes
	Grace    time.Duration // how long a problem lasts before an email; default 3 minutes
	Log      *slog.Logger
	Now      func() time.Time
}

// Run checks until ctx ends.
func (r *Runner) Run(ctx context.Context) {
	every := r.Every
	if every == 0 {
		every = 2 * time.Minute
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		r.Once(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// kinds of alert, as the tenant turns them on and off (store.AlertKinds).
var kinds = map[string]string{
	"Connector": "connector", "Legacy site": "legacy_site", "Isolation": "isolation",
	"Certificate": "certificate", expiryCheck: "certificate_expiry",
}

func kindOf(c change) string {
	if !c.down {
		return "recovered"
	}
	return kinds[c.check]
}

// change is one line of an alert email.
type change struct {
	site, domain, check, detail string
	down                        bool
}

// Once checks every tenant and sends the emails due.
func (r *Runner) Once(ctx context.Context) {
	tenants, err := r.Store.Tenants(ctx, nil)
	if err != nil {
		r.Log.Error("alerts: listing tenants", slog.String("error", err.Error()))
		return
	}
	for _, t := range tenants {
		if ctx.Err() != nil {
			return
		}
		cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		r.tenant(cctx, t)
		cancel()
	}
}

func (r *Runner) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *Runner) tenant(ctx context.Context, t store.Tenant) {
	settings, err := r.Store.Alerts(ctx, t.ID)
	if err != nil || !settings.Enabled {
		return
	}
	sites, _, err := r.Statuses(ctx, t.ID)
	if err != nil {
		r.Log.Warn("alerts: site status", slog.Group("detail", slog.String("tenant", t.ID), slog.String("error", err.Error())))
		return
	}
	var changes []change
	for _, s := range sites {
		for _, c := range r.site(ctx, s) {
			if settings.Wants(kindOf(c)) { // the state is followed even when muted
				changes = append(changes, c)
			}
		}
	}
	if len(changes) == 0 {
		return
	}
	to := r.recipients(ctx, t.ID, settings)
	if len(to) == 0 {
		return
	}
	msg := compose(t.Name, changes, r.PanelURL, t.ID)
	for _, addr := range to {
		msg.To = addr
		if err := r.Mail.Send(ctx, msg); err != nil {
			r.Log.Error("alert email not sent", slog.String("error", err.Error()))
		}
	}
	tenant := t.ID
	_ = r.Store.Record(ctx, nil, &tenant, "alert.sent", "", map[string]any{"changes": len(changes), "recipients": len(to)})
}

// site compares a site's checks with what was seen and sent before.
func (r *Runner) site(ctx context.Context, s health.Site) []change {
	known, err := r.Store.SiteAlerts(ctx, s.App)
	if err != nil {
		return nil
	}
	grace := r.Grace
	if grace == 0 {
		grace = 3 * time.Minute
	}
	now := r.now()
	var out []change
	for _, c := range s.Checks {
		if !Watched[c.Name] || (c.State != health.Down && c.State != health.OK) {
			continue // unknown or degraded: nothing to say yet
		}
		k := known[c.Name]
		switch {
		case c.State == health.OK && k.Notified == health.Down:
			k = store.SiteAlert{State: health.OK, Since: now, Notified: health.OK}
			out = append(out, change{s.Name, s.PublicDomain, c.Name, c.Detail, false})
		case k.State != c.State:
			k = store.SiteAlert{State: c.State, Since: now, Notified: k.Notified}
		case c.State == health.Down && k.Notified != health.Down && now.Sub(k.Since) >= grace:
			k.Notified = health.Down
			out = append(out, change{s.Name, s.PublicDomain, c.Name, c.Detail, true})
		default:
			continue
		}
		if err := r.Store.SaveSiteAlert(ctx, s.App, c.Name, k); err != nil {
			return nil
		}
	}
	if c, ok := r.expiry(ctx, s, known[expiryCheck], now); ok {
		out = append(out, c)
	}
	return out
}

// expiry sends one reminder when the certificate crosses each of Reminders
// (30, 14, 7, 1 days before its end); a renewed certificate starts again.
func (r *Runner) expiry(ctx context.Context, s health.Site, k store.SiteAlert, now time.Time) (change, bool) {
	if s.CertificateExpires == nil {
		return change{}, false
	}
	left := int(s.CertificateExpires.Sub(now).Hours() / 24)
	due := ""
	for _, d := range Reminders {
		if left < d && left >= 0 {
			due = strconv.Itoa(d)
		}
	}
	if due == k.Notified || (due == "" && k.Notified == "") {
		return change{}, false
	}
	k = store.SiteAlert{State: due, Since: now, Notified: due}
	if err := r.Store.SaveSiteAlert(ctx, s.App, expiryCheck, k); err != nil || due == "" {
		return change{}, false // renewed: nothing to say
	}
	return change{s.Name, s.PublicDomain, expiryCheck, fmt.Sprintf("the certificate ends on %s (%d days): upload the renewed one in Site → Certificate",
		s.CertificateExpires.UTC().Format(time.DateOnly), left), true}, true
}

func (r *Runner) recipients(ctx context.Context, tenant string, s store.AlertSettings) []string {
	seen := map[string]bool{}
	var out []string
	add := func(e string) {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" && !seen[e] {
			seen[e] = true
			out = append(out, e)
		}
	}
	if users, err := r.Store.Users(ctx, tenant); err == nil {
		for _, u := range users {
			if u.TenantRole != nil && *u.TenantRole == "admin" {
				add(u.Email)
			}
		}
	}
	for _, e := range s.ExtraEmails {
		add(e)
	}
	sort.Strings(out)
	return out
}

func compose(tenantName string, cs []change, panel, tenant string) mail.Message {
	down := 0
	var b strings.Builder
	for _, c := range cs {
		if c.down {
			down++
			fmt.Fprintf(&b, "PROBLEM  %s (%s), %s: %s\n", c.site, c.domain, strings.ToLower(c.check), c.detail)
		} else {
			fmt.Fprintf(&b, "WORKING  %s (%s), %s: %s\n", c.site, c.domain, strings.ToLower(c.check), c.detail)
		}
	}
	subject := fmt.Sprintf("Equalizer: %d site problem(s) for %s", down, tenantName)
	if down == 0 {
		subject = "Equalizer: sites of " + tenantName + " working again"
	}
	body := "Hello,\n\nthe status of your protected sites changed:\n\n" + b.String() +
		"\nDetails on the Status page: " + strings.TrimSuffix(panel, "/") + "/t/" + tenant + "/status\n\n" +
		"You receive this as an administrator of " + tenantName + " (Settings → Alerts).\n"
	return mail.Message{Subject: subject, Body: body}
}
