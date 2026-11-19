package store

import (
	"context"
	"time"
)

// CreateReset stores a password reset link for a user; earlier unused links
// of the same user stop working.
func (s *Store) CreateReset(ctx context.Context, userID int64, tokenHash string, expires time.Time) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM password_resets WHERE user_id = $1 AND used_at IS NULL`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO password_resets (user_id, token_sha256, expires_at) VALUES ($1, $2, $3)`,
		userID, tokenHash, expires); err != nil {
		return err
	}
	return tx.Commit()
}

// ResetValid reports whether a reset link works: unused, not expired, for
// an active user.
func (s *Store) ResetValid(ctx context.Context, tokenHash string) (bool, error) {
	var ok bool
	err := s.DB.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM password_resets r JOIN users u ON u.id = r.user_id
		WHERE r.token_sha256 = $1 AND r.used_at IS NULL AND r.expires_at > now() AND NOT u.disabled)`, tokenHash).Scan(&ok)
	return ok, err
}

// ConsumeReset uses a reset link once and returns its user; ErrNotFound
// when it does not work.
func (s *Store) ConsumeReset(ctx context.Context, tokenHash string) (int64, *string, error) {
	var id int64
	var tenant *string
	err := s.DB.QueryRowContext(ctx, `
		UPDATE password_resets r SET used_at = now() FROM users u
		WHERE u.id = r.user_id AND r.token_sha256 = $1 AND r.used_at IS NULL AND r.expires_at > now() AND NOT u.disabled
		RETURNING u.id, u.tenant_id`, tokenHash).Scan(&id, &tenant)
	return id, tenant, translate(err)
}
