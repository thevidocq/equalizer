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
}

// Alerts returns a tenant's settings: on, no extra address, when never set.
func (s *Store) Alerts(ctx context.Context, tenant string) (AlertSettings, error) {
	out := AlertSettings{Enabled: true, ExtraEmails: []string{}}
	err := s.DB.QueryRowContext(ctx, `SELECT enabled, extra_emails FROM tenant_alerts WHERE tenant_id = $1`, tenant).
		Scan(&out.Enabled, pq.Array(&out.ExtraEmails))
	if errors.Is(err, sql.ErrNoRows) {
		return out, nil
	}
	if out.ExtraEmails == nil {
		out.ExtraEmails = []string{}
	}
	return out, err
}

// SetAlerts saves a tenant's settings.
func (s *Store) SetAlerts(ctx context.Context, tenant string, a AlertSettings) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO tenant_alerts (tenant_id, enabled, extra_emails) VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id) DO UPDATE SET enabled = $2, extra_emails = $3`, tenant, a.Enabled, pq.Array(a.ExtraEmails))
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
