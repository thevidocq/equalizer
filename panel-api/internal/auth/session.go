package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"
)

// CookieName is the panel session cookie (HTTPS or localhost only).
const CookieName = "__Host-eq_panel"

// ErrNoSession reports a missing or expired session.
var ErrNoSession = errors.New("no session")

// Sessions stores panel sessions in PostgreSQL (only the id's SHA-256).
type Sessions struct {
	DB  *sql.DB
	TTL time.Duration
}

func idHash(id string) string {
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:])
}

// Create starts a session for a user and returns the cookie value. mfaOK
// says whether the second step is already done (or not required).
func (s *Sessions) Create(ctx context.Context, userID int64, mfaOK bool) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	id := base64.RawURLEncoding.EncodeToString(raw)
	_, err := s.DB.ExecContext(ctx, `INSERT INTO panel_sessions (id_hash, user_id, expires_at, mfa_ok) VALUES ($1, $2, $3, $4)`,
		idHash(id), userID, time.Now().Add(s.TTL), mfaOK)
	return id, err
}

// Upgrade replaces a session that passed the second step with a new one
// (new id: the pre-MFA id is worthless).
func (s *Sessions) Upgrade(ctx context.Context, oldID string, userID int64) (string, error) {
	if err := s.Delete(ctx, oldID); err != nil {
		return "", err
	}
	return s.Create(ctx, userID, true)
}

// EndAll ends every session of a user except keep ("" ends all).
func (s *Sessions) EndAll(ctx context.Context, userID int64, keep string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM panel_sessions WHERE user_id = $1 AND id_hash <> $2`, userID, idHash(keep))
	return err
}

// User returns the user of a valid session.
func (s *Sessions) User(ctx context.Context, id string) (*User, error) {
	if id == "" || len(id) > 64 {
		return nil, ErrNoSession
	}
	u := &User{}
	err := s.DB.QueryRowContext(ctx, `
		SELECT u.id, u.email, u.name, u.platform_role, u.tenant_id, u.tenant_role, u.mfa_secret IS NOT NULL, s.mfa_ok
		FROM panel_sessions s JOIN users u ON u.id = s.user_id
		WHERE s.id_hash = $1 AND s.expires_at > now() AND NOT u.disabled`, idHash(id)).
		Scan(&u.ID, &u.Email, &u.Name, &u.PlatformRole, &u.Tenant, &u.TenantRole, &u.MFAEnabled, &u.MFAVerified)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoSession
	}
	return u, err
}

// Delete ends a session.
func (s *Sessions) Delete(ctx context.Context, id string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM panel_sessions WHERE id_hash = $1`, idHash(id))
	return err
}

// Purge removes expired sessions.
func (s *Sessions) Purge(ctx context.Context) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM panel_sessions WHERE expires_at <= now()`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
