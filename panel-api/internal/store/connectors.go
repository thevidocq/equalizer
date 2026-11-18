package store

import (
	"context"
	"time"
)

// Connector is a registered connector (the token is stored only as SHA-256).
type Connector struct {
	ID          string     `json:"id"`
	App         string     `json:"app"`
	Tenant      string     `json:"tenant"`
	TokenSHA256 string     `json:"-"`
	CreatedAt   time.Time  `json:"created_at"`
	RotatedAt   *time.Time `json:"rotated_at"`
}

// CreateConnector registers a connector.
func (s *Store) CreateConnector(ctx context.Context, app, id, tokenHash string) (Connector, error) {
	c := Connector{ID: id, App: app}
	err := s.DB.QueryRowContext(ctx, `
		INSERT INTO connectors (id, app_id, token_sha256) VALUES ($1, $2, $3)
		RETURNING created_at, (SELECT tenant_id FROM apps WHERE id = $2)`, id, app, tokenHash).Scan(&c.CreatedAt, &c.Tenant)
	return c, translate(err)
}

// RotateConnector replaces the token hash of a connector of the app.
func (s *Store) RotateConnector(ctx context.Context, app, id, tokenHash string) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE connectors SET token_sha256 = $3, rotated_at = now() WHERE id = $1 AND app_id = $2`, id, app, tokenHash)
	return affected(res, err)
}

// DeleteConnector removes a connector of the app.
func (s *Store) DeleteConnector(ctx context.Context, app, id string) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM connectors WHERE id = $1 AND app_id = $2`, id, app)
	return affected(res, err)
}

// Connectors lists the connectors of one app, or of every app when app is "".
func (s *Store) Connectors(ctx context.Context, app string) ([]Connector, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT c.id, c.app_id, a.tenant_id, c.token_sha256, c.created_at, c.rotated_at
		FROM connectors c JOIN apps a ON a.id = c.app_id
		WHERE $1 = '' OR c.app_id = $1 ORDER BY c.id`, app)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Connector{}
	for rows.Next() {
		var c Connector
		if err := rows.Scan(&c.ID, &c.App, &c.Tenant, &c.TokenSHA256, &c.CreatedAt, &c.RotatedAt); err != nil {
			return nil, err
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
