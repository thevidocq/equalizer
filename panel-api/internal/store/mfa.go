package store

import (
	"context"

	"github.com/lib/pq"
)

// MFAState is what the second login step needs about a user.
type MFAState struct {
	Secret   []byte // encrypted; nil when not enrolled
	Pending  []byte // encrypted; nil when no enrolment is in progress
	LastStep int64
	Recovery []string // hashes
	PassHash string
	TenantID *string
}

// MFA reads the MFA state of an active user.
func (s *Store) MFA(ctx context.Context, userID int64) (MFAState, error) {
	var m MFAState
	err := s.DB.QueryRowContext(ctx, `
		SELECT mfa_secret, mfa_pending, mfa_last_step, recovery_codes, password_hash, tenant_id
		FROM users WHERE id = $1 AND NOT disabled`, userID).
		Scan(&m.Secret, &m.Pending, &m.LastStep, pq.Array(&m.Recovery), &m.PassHash, &m.TenantID)
	return m, translate(err)
}

// SetMFAPending stores a secret waiting for its first code.
func (s *Store) SetMFAPending(ctx context.Context, userID int64, sealed []byte) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE users SET mfa_pending = $2 WHERE id = $1`, userID, sealed)
	return affected(res, err)
}

// EnableMFA activates the pending secret with the accepted step and the
// recovery code hashes.
func (s *Store) EnableMFA(ctx context.Context, userID int64, sealed []byte, step int64, recovery []string) error {
	res, err := s.DB.ExecContext(ctx, `
		UPDATE users SET mfa_secret = $2, mfa_pending = NULL, mfa_last_step = $3, recovery_codes = $4 WHERE id = $1`,
		userID, sealed, step, pq.Array(recovery))
	return affected(res, err)
}

// UseStep records an accepted step; false if a newer or equal one was
// already used (a concurrent replay).
func (s *Store) UseStep(ctx context.Context, userID int64, step int64) (bool, error) {
	res, err := s.DB.ExecContext(ctx, `UPDATE users SET mfa_last_step = $2 WHERE id = $1 AND mfa_last_step < $2`, userID, step)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// UseRecoveryCode consumes a recovery code; false if it is not (or no longer) valid.
func (s *Store) UseRecoveryCode(ctx context.Context, userID int64, hash string) (bool, error) {
	res, err := s.DB.ExecContext(ctx, `
		UPDATE users SET recovery_codes = array_remove(recovery_codes, $2)
		WHERE id = $1 AND $2 = ANY(recovery_codes)`, userID, hash)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// ResetMFA removes a tenant user's authenticator.
func (s *Store) ResetMFA(ctx context.Context, tenant string, userID int64) error {
	res, err := s.DB.ExecContext(ctx, `
		UPDATE users SET mfa_secret = NULL, mfa_pending = NULL, recovery_codes = '{}'
		WHERE id = $1 AND tenant_id = $2 AND NOT disabled`, userID, tenant)
	return affected(res, err)
}

// SetPassword replaces a user's password hash.
func (s *Store) SetPassword(ctx context.Context, userID int64, hash string) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, userID, hash)
	return affected(res, err)
}

// Deactivate disables a tenant user and ends their sessions.
func (s *Store) Deactivate(ctx context.Context, tenant string, userID int64) (string, error) {
	var email string
	err := s.DB.QueryRowContext(ctx, `
		UPDATE users SET disabled = true WHERE id = $1 AND tenant_id = $2 AND NOT disabled RETURNING email`, userID, tenant).Scan(&email)
	if err != nil {
		return "", translate(err)
	}
	_, err = s.DB.ExecContext(ctx, `DELETE FROM panel_sessions WHERE user_id = $1`, userID)
	return email, err
}

// EndSessions ends every session of a user.
func (s *Store) EndSessions(ctx context.Context, userID int64) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM panel_sessions WHERE user_id = $1`, userID)
	return err
}
