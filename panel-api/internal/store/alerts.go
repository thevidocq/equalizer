package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lib/pq"
)

// AlertSettings are a tenant's email alerts.
type AlertSettings struct {
	Enabled     bool     `json:"enabled"`
	ExtraEmails []string `json:"extra_emails"`
	// Kinds says which kinds of alert are emailed (AlertKinds); all when
	// missing.
	Kinds map[string]bool `json:"kinds"`
}

// AlertKinds are the kinds of alert a tenant turns on or off one by one.
var AlertKinds = []string{"connector", "legacy_site", "isolation", "certificate", "certificate_expiry", "recovered", "draft_ready"}

// Wants reports whether a kind of alert is emailed.
func (a AlertSettings) Wants(kind string) bool {
	on, set := a.Kinds[kind]
	return !set || on
}

func kindsFrom(muted []string) map[string]bool {
	out := map[string]bool{}
	for _, k := range AlertKinds {
		out[k] = true
	}
	for _, k := range muted {
		if _, ok := out[k]; ok {
			out[k] = false
		}
	}
	return out
}

func mutedOf(kinds map[string]bool) []string {
	out := []string{}
	for _, k := range AlertKinds {
		if on, set := kinds[k]; set && !on {
			out = append(out, k)
		}
	}
	return out
}

// Alerts returns a tenant's settings: on, no extra address, when never set.
func (s *Store) Alerts(ctx context.Context, tenant string) (AlertSettings, error) {
	out := AlertSettings{Enabled: true, ExtraEmails: []string{}, Kinds: kindsFrom(nil)}
	var muted []string
	err := s.DB.QueryRowContext(ctx, `SELECT enabled, extra_emails, muted FROM tenant_alerts WHERE tenant_id = $1`, tenant).
		Scan(&out.Enabled, pq.Array(&out.ExtraEmails), pq.Array(&muted))
	if errors.Is(err, sql.ErrNoRows) {
		return out, nil
	}
	out.Kinds = kindsFrom(muted)
	if out.ExtraEmails == nil {
		out.ExtraEmails = []string{}
	}
	return out, err
}

// SetAlerts saves a tenant's settings.
func (s *Store) SetAlerts(ctx context.Context, tenant string, a AlertSettings) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO tenant_alerts (tenant_id, enabled, extra_emails, muted) VALUES ($1, $2, $3, $4)
		ON CONFLICT (tenant_id) DO UPDATE SET enabled = $2, extra_emails = $3, muted = $4`,
		tenant, a.Enabled, pq.Array(a.ExtraEmails), pq.Array(mutedOf(a.Kinds)))
	return translate(err)
}

// SiteAlert is the last state seen of one check of a site.
type SiteAlert struct {
	State    string
	Since    time.Time
	Notified string // the state last sent by email
}

// SiteAlerts returns the states known for a site's checks.
func (s *Store) SiteAlerts(ctx context.Context, app string) (map[string]SiteAlert, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT check_name, state, since, notified FROM site_alerts WHERE app_id = $1`, app)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]SiteAlert{}
	for rows.Next() {
		var name string
		var a SiteAlert
		if err := rows.Scan(&name, &a.State, &a.Since, &a.Notified); err != nil {
			return nil, err
		}
		out[name] = a
	}
	return out, rows.Err()
}

// SaveSiteAlert stores the state of one check of a site.
func (s *Store) SaveSiteAlert(ctx context.Context, app, check string, a SiteAlert) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO site_alerts (app_id, check_name, state, since, notified) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (app_id, check_name) DO UPDATE SET state = $3, since = $4, notified = $5`,
		app, check, a.State, a.Since, a.Notified)
	return err
}
