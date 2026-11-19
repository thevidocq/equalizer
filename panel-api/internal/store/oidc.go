package store

import (
	"context"
	"database/sql"
	"errors"
)

// OIDCSecret is a site's sealed client secret.
type OIDCSecret struct {
	App    string
	Sealed []byte
}

// SetOIDCSecret saves the sealed client secret of a site.
func (s *Store) SetOIDCSecret(ctx context.Context, app string, sealed []byte) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO app_oidc (app_id, client_secret) VALUES ($1, $2)
		ON CONFLICT (app_id) DO UPDATE SET client_secret = $2, updated_at = now()`, app, sealed)
	return translate(err)
}

// DeleteOIDCSecret removes it.
func (s *Store) DeleteOIDCSecret(ctx context.Context, app string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM app_oidc WHERE app_id = $1`, app)
	return err
}

// HasOIDCSecret reports whether a site has one.
func (s *Store) HasOIDCSecret(ctx context.Context, app string) (bool, error) {
	var one int
	err := s.DB.QueryRowContext(ctx, `SELECT 1 FROM app_oidc WHERE app_id = $1`, app).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// OIDCSecrets returns every sealed secret (for the gateways).
func (s *Store) OIDCSecrets(ctx context.Context) ([]OIDCSecret, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT app_id, client_secret FROM app_oidc ORDER BY app_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OIDCSecret
	for rows.Next() {
		var o OIDCSecret
		if err := rows.Scan(&o.App, &o.Sealed); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
