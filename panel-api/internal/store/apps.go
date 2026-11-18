package store

import (
	"context"
	"time"
)

// App is a protected site.
type App struct {
	ID           string    `json:"id"`
	Tenant       string    `json:"tenant"`
	Name         string    `json:"name"`
	PublicDomain string    `json:"public_domain"`
	CreatedAt    time.Time `json:"created_at"`
	Published    *int      `json:"published_version"` // nil: no published map
	Exposure     Exposure  `json:"exposure"`
}

// Exposure is the result of the last exposure check of a site.
type Exposure struct {
	LegacyIP  *string    `json:"legacy_ip"`
	Status    string     `json:"status"`
	Reason    *string    `json:"reason"`
	CheckedAt *time.Time `json:"checked_at"`
}

// CreateApp adds a protected site.
func (s *Store) CreateApp(ctx context.Context, a App) (App, error) {
	err := s.DB.QueryRowContext(ctx, `
		INSERT INTO apps (id, tenant_id, name, public_domain) VALUES ($1, $2, $3, lower($4)) RETURNING public_domain, created_at, exposure_status`,
		a.ID, a.Tenant, a.Name, a.PublicDomain).Scan(&a.PublicDomain, &a.CreatedAt, &a.Exposure.Status)
	return a, translate(err)
}

const appColumns = `a.id, a.tenant_id, a.name, a.public_domain, a.created_at,
	(SELECT version FROM map_versions m WHERE m.app_id = a.id AND m.status = 'published'),
	a.legacy_ip, a.exposure_status, a.exposure_reason, a.exposure_checked_at`

type scanner interface{ Scan(...any) error }

func scanApp(row scanner) (App, error) {
	var a App
	err := row.Scan(&a.ID, &a.Tenant, &a.Name, &a.PublicDomain, &a.CreatedAt, &a.Published,
		&a.Exposure.LegacyIP, &a.Exposure.Status, &a.Exposure.Reason, &a.Exposure.CheckedAt)
	return a, err
}

// Apps lists the sites of a tenant.
func (s *Store) Apps(ctx context.Context, tenant string) ([]App, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+appColumns+` FROM apps a WHERE a.tenant_id = $1 ORDER BY a.id`, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []App{}
	for rows.Next() {
		a, err := scanApp(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// App returns one site.
func (s *Store) App(ctx context.Context, id string) (App, error) {
	a, err := scanApp(s.DB.QueryRowContext(ctx, `SELECT `+appColumns+` FROM apps a WHERE a.id = $1`, id))
	return a, translate(err)
}

// SetLegacyIP sets (or clears with nil) the old public IP of a site. The
// exposure status restarts: pending until the next check.
func (s *Store) SetLegacyIP(ctx context.Context, app string, ip *string) error {
	status := "pending"
	if ip == nil {
		status = "not_configured"
	}
	res, err := s.DB.ExecContext(ctx, `
		UPDATE apps SET legacy_ip = $2, exposure_status = $3, exposure_reason = NULL, exposure_checked_at = NULL
		WHERE id = $1`, app, ip, status)
	return affected(res, err)
}

// ExposureTarget is what the exposure checker probes.
type ExposureTarget struct {
	App, Tenant, Domain, IP, Status string
	CheckedAt                       *time.Time
}

// ExposureTargets lists the sites with an old public IP.
func (s *Store) ExposureTargets(ctx context.Context) ([]ExposureTarget, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, tenant_id, public_domain, legacy_ip, exposure_status, exposure_checked_at
		FROM apps WHERE legacy_ip IS NOT NULL ORDER BY exposure_checked_at NULLS FIRST, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ExposureTarget
	for rows.Next() {
		var t ExposureTarget
		if err := rows.Scan(&t.App, &t.Tenant, &t.Domain, &t.IP, &t.Status, &t.CheckedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// SaveExposure stores a check result, unless the IP changed meanwhile. It
// returns the previous status.
func (s *Store) SaveExposure(ctx context.Context, app, ip, status, reason string) (string, error) {
	var before string
	err := s.DB.QueryRowContext(ctx, `
		WITH old AS (SELECT exposure_status FROM apps WHERE id = $1 AND legacy_ip = $2 FOR UPDATE)
		UPDATE apps SET exposure_status = $3, exposure_reason = $4, exposure_checked_at = now()
		WHERE id = $1 AND legacy_ip = $2
		RETURNING (SELECT exposure_status FROM old)`, app, ip, status, reason).Scan(&before)
	return before, translate(err)
}
