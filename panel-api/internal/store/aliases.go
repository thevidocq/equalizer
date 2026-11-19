package store

import (
	"context"
	"database/sql"
)

// DomainTaken reports whether a domain is already used by a site other than
// app, as its public domain or as an alias.
func (s *Store) DomainTaken(ctx context.Context, domain, app string) (bool, error) {
	var taken bool
	err := s.DB.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM apps WHERE public_domain = lower($1) AND id <> $2)
		    OR EXISTS (SELECT 1 FROM app_aliases WHERE domain = lower($1) AND app_id <> $2)`, domain, app).Scan(&taken)
	return taken, err
}

// SetAliases replaces the other domains of a site.
func (s *Store) SetAliases(ctx context.Context, app string, domains []string) error {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM app_aliases WHERE app_id = $1`, app); err != nil {
		return err
	}
	for _, d := range domains {
		if _, err := tx.ExecContext(ctx, `INSERT INTO app_aliases (domain, app_id) VALUES (lower($1), $2)`, d, app); err != nil {
			return translate(err)
		}
	}
	return translate(tx.Commit())
}

// Republish replaces the document and signature of the published version
// (same version: only meta changed, e.g. the site's other domains).
func (s *Store) Republish(ctx context.Context, app string, version int, doc []byte, payload, signature, keyID string) error {
	res, err := s.DB.ExecContext(ctx, `
		UPDATE map_versions SET document = $3, payload = $4, signature = $5, key_id = $6
		WHERE app_id = $1 AND version = $2 AND status = 'published'`, app, version, doc, payload, signature, keyID)
	return affected(res, err)
}
