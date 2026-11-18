package store

import (
	"context"
	"time"

	"github.com/lib/pq"
)

// Connector is a registered connector of a tenant (the token is stored only
// as SHA-256). One connector serves every site of the servers it reaches.
type Connector struct {
	ID          string     `json:"id"`
	Tenant      string     `json:"tenant"`
	UsedBy      []string   `json:"used_by"` // sites whose published map uses it
	TokenSHA256 string     `json:"-"`
	CreatedAt   time.Time  `json:"created_at"`
	RotatedAt   *time.Time `json:"rotated_at"`
}

// CreateConnector registers a connector of the tenant.
func (s *Store) CreateConnector(ctx context.Context, tenant, id, tokenHash string) (Connector, error) {
	c := Connector{ID: id, Tenant: tenant, UsedBy: []string{}}
	err := s.DB.QueryRowContext(ctx, `
		INSERT INTO connectors (id, tenant_id, token_sha256) VALUES ($1, $2, $3) RETURNING created_at`,
		id, tenant, tokenHash).Scan(&c.CreatedAt)
	return c, translate(err)
}

// RotateConnector replaces the token hash of a connector of the tenant.
func (s *Store) RotateConnector(ctx context.Context, tenant, id, tokenHash string) error {
	res, err := s.DB.ExecContext(ctx, `
		UPDATE connectors SET token_sha256 = $3, rotated_at = now() WHERE id = $1 AND tenant_id = $2`, id, tenant, tokenHash)
	return affected(res, err)
}

// DeleteConnector removes a connector of the tenant. ErrState while a
// published map uses it: removing it would take the site offline.
func (s *Store) DeleteConnector(ctx context.Context, tenant, id string) error {
	cs, err := s.connectors(ctx, `WHERE c.id = $1 AND c.tenant_id = $2`, id, tenant)
	if err != nil {
		return err
	}
	if len(cs) == 0 {
		return ErrNotFound
	}
	if len(cs[0].UsedBy) > 0 {
		return ErrState
	}
	res, err := s.DB.ExecContext(ctx, `DELETE FROM connectors WHERE id = $1 AND tenant_id = $2`, id, tenant)
	return affected(res, err)
}

// Connectors lists the connectors of a tenant.
func (s *Store) Connectors(ctx context.Context, tenant string) ([]Connector, error) {
	return s.connectors(ctx, `WHERE c.tenant_id = $1`, tenant)
}

// AllConnectors lists every connector (for the relay).
func (s *Store) AllConnectors(ctx context.Context) ([]Connector, error) {
	return s.connectors(ctx, ``)
}

// ConnectorTenant returns the tenant owning a connector.
func (s *Store) ConnectorTenant(ctx context.Context, id string) (string, error) {
	var t string
	err := s.DB.QueryRowContext(ctx, `SELECT tenant_id FROM connectors WHERE id = $1`, id).Scan(&t)
	return t, translate(err)
}

func (s *Store) connectors(ctx context.Context, where string, args ...any) ([]Connector, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT c.id, c.tenant_id, c.token_sha256, c.created_at, c.rotated_at,
			ARRAY(SELECT m.app_id FROM map_versions m JOIN apps a ON a.id = m.app_id
				WHERE m.status = 'published' AND a.tenant_id = c.tenant_id
				AND m.document->'legacy'->'reach'->>'connector_id' = c.id ORDER BY m.app_id)
		FROM connectors c `+where+` ORDER BY c.id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Connector{}
	for rows.Next() {
		var c Connector
		if err := rows.Scan(&c.ID, &c.Tenant, &c.TokenSHA256, &c.CreatedAt, &c.RotatedAt, pq.Array(&c.UsedBy)); err != nil {
			return nil, err
		}
		if c.UsedBy == nil {
			c.UsedBy = []string{}
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func affected(res interface{ RowsAffected() (int64, error) }, err error) error {
	if err != nil {
		return translate(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
