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
}

// CreateApp adds a protected site.
func (s *Store) CreateApp(ctx context.Context, a App) (App, error) {
	err := s.DB.QueryRowContext(ctx, `
		INSERT INTO apps (id, tenant_id, name, public_domain) VALUES ($1, $2, $3, lower($4)) RETURNING public_domain, created_at`,
		a.ID, a.Tenant, a.Name, a.PublicDomain).Scan(&a.PublicDomain, &a.CreatedAt)
	return a, translate(err)
}

const appColumns = `a.id, a.tenant_id, a.name, a.public_domain, a.created_at,
	(SELECT version FROM map_versions m WHERE m.app_id = a.id AND m.status = 'published')`

// Apps lists the sites of a tenant.
func (s *Store) Apps(ctx context.Context, tenant string) ([]App, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+appColumns+` FROM apps a WHERE a.tenant_id = $1 ORDER BY a.id`, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []App{}
	for rows.Next() {
		var a App
		if err := rows.Scan(&a.ID, &a.Tenant, &a.Name, &a.PublicDomain, &a.CreatedAt, &a.Published); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// App returns one site.
func (s *Store) App(ctx context.Context, id string) (App, error) {
	var a App
	err := s.DB.QueryRowContext(ctx, `SELECT `+appColumns+` FROM apps a WHERE a.id = $1`, id).
		Scan(&a.ID, &a.Tenant, &a.Name, &a.PublicDomain, &a.CreatedAt, &a.Published)
	return a, translate(err)
}
