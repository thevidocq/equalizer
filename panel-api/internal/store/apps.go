package store

import (
	"context"
	"database/sql"
	"github.com/lib/pq"
	"time"
)

// App is a protected site.
type App struct {
	ID           string        `json:"id"`
	Tenant       string        `json:"tenant"`
	Name         string        `json:"name"`
	PublicDomain string        `json:"public_domain"`
	Aliases      []string      `json:"aliases"`
	CreatedAt    time.Time     `json:"created_at"`
	Published    *int          `json:"published_version"` // nil: no published map
	ReachMode    *string       `json:"reach_mode"`        // connector or direct, from the published map; nil: none
	Kind         *string       `json:"kind"`              // wordpress, joomla, ... or nil
	Schedule     string        `json:"discovery_schedule"`
	DiscoveryAt  DiscoveryTime `json:"discovery_time"`
	Exposure     Exposure      `json:"exposure"`
	TLS          SiteTLS       `json:"tls"`
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
		INSERT INTO apps (id, tenant_id, name, public_domain, kind) VALUES ($1, $2, $3, lower($4), $5)
		RETURNING public_domain, created_at, exposure_status, discovery_schedule`,
		a.ID, a.Tenant, a.Name, a.PublicDomain, a.Kind).Scan(&a.PublicDomain, &a.CreatedAt, &a.Exposure.Status, &a.Schedule)
	return a, translate(err)
}

const appColumns = `a.id, a.tenant_id, a.name, a.public_domain, a.created_at,
	(SELECT version FROM map_versions m WHERE m.app_id = a.id AND m.status = 'published'),
	(SELECT m.document->'legacy'->'reach'->>'mode' FROM map_versions m WHERE m.app_id = a.id AND m.status = 'published'),
	a.legacy_ip, a.exposure_status, a.exposure_reason, a.exposure_checked_at, a.kind, a.discovery_schedule,
	ARRAY(SELECT x.domain FROM app_aliases x WHERE x.app_id = a.id ORDER BY x.domain), a.public_scheme,
	c.subject, c.issuer, c.domains, c.not_before, c.not_after, c.fingerprint, c.uploaded_at, c.chain_pem,
	(SELECT u.email FROM users u WHERE u.id = c.uploaded_by),
	a.discovery_hour, a.discovery_weekday, a.discovery_zone, a.discovery_set_at,
	(SELECT max(d.created_at) FROM discoveries d WHERE d.app_id = a.id)`

// appFrom is the FROM of app queries: the site and its certificate, if any.
const appFrom = ` FROM apps a LEFT JOIN app_certificates c ON c.app_id = a.id`

type scanner interface{ Scan(...any) error }

func scanApp(row scanner) (App, error) {
	var a App
	var (
		subject, issuer, fingerprint, chain sql.NullString
		domains                             []string
		notBefore, notAfter, uploadedAt     sql.NullTime
		uploadedBy                          *string
	)
	err := row.Scan(&a.ID, &a.Tenant, &a.Name, &a.PublicDomain, &a.CreatedAt, &a.Published, &a.ReachMode,
		&a.Exposure.LegacyIP, &a.Exposure.Status, &a.Exposure.Reason, &a.Exposure.CheckedAt, &a.Kind, &a.Schedule,
		pq.Array(&a.Aliases), &a.TLS.PublicScheme, &subject, &issuer, pq.Array(&domains), &notBefore, &notAfter,
		&fingerprint, &uploadedAt, &chain, &uploadedBy,
		&a.DiscoveryAt.Hour, &a.DiscoveryAt.Weekday, &a.DiscoveryAt.Zone, &a.DiscoveryAt.setAt, &a.DiscoveryAt.Last)
	if a.Aliases == nil {
		a.Aliases = []string{}
	}
	if subject.Valid {
		a.TLS.Certificate = &Certificate{Subject: subject.String, Issuer: issuer.String, Domains: domains,
			NotBefore: notBefore.Time, NotAfter: notAfter.Time, Fingerprint: fingerprint.String,
			UploadedAt: uploadedAt.Time, UploadedBy: uploadedBy, ChainPEM: chain.String}
	}
	fillTLS(&a, time.Now())
	if a.Schedule == "daily" || a.Schedule == "weekly" {
		next := a.DiscoveryAt.NextSlot(a.Schedule, time.Now())
		a.DiscoveryAt.Next = &next
	}
	return a, err
}

// Apps lists the sites of a tenant.
func (s *Store) Apps(ctx context.Context, tenant string) ([]App, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+appColumns+appFrom+` WHERE a.tenant_id = $1 ORDER BY a.id`, tenant)
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
	a, err := scanApp(s.DB.QueryRowContext(ctx, `SELECT `+appColumns+appFrom+` WHERE a.id = $1`, id))
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

// SetNeverPublic records that the site never had a public address (true),
// or forgets it (false): no old IP, nothing to probe.
func (s *Store) SetNeverPublic(ctx context.Context, app string, never bool) error {
	status := "not_configured"
	if never {
		status = "never_public"
	}
	res, err := s.DB.ExecContext(ctx, `
		UPDATE apps SET legacy_ip = NULL, exposure_status = $2, exposure_reason = NULL, exposure_checked_at = NULL
		WHERE id = $1`, app, status)
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
		FROM apps WHERE legacy_ip IS NOT NULL
			-- reached directly: the probe comes from the address the site accepts
			AND NOT EXISTS (SELECT 1 FROM map_versions m WHERE m.app_id = apps.id AND m.status = 'published'
				AND m.document->'legacy'->'reach'->>'mode' = 'direct')
		ORDER BY exposure_checked_at NULLS FIRST, id`)
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
