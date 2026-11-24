package store

import (
	"context"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
)

// Tenant is a customer of the platform.
type Tenant struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateTenant adds a tenant.
func (s *Store) CreateTenant(ctx context.Context, id, name string) (Tenant, error) {
	t := Tenant{ID: id, Name: name}
	err := s.DB.QueryRowContext(ctx, `INSERT INTO tenants (id, name) VALUES ($1, $2) RETURNING created_at`, id, name).Scan(&t.CreatedAt)
	return t, translate(err)
}

// Tenants lists tenants; only is non-nil to restrict to one tenant.
func (s *Store) Tenants(ctx context.Context, only *string) ([]Tenant, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, name, created_at FROM tenants WHERE $1::text IS NULL OR id = $1 ORDER BY id`, only)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Tenant{}
	for rows.Next() {
		var t Tenant
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// NewUser is a user to create. Exactly one of PlatformRole or Tenant+TenantRole.
type NewUser struct {
	Email        string
	Name         string
	PasswordHash string
	PlatformRole *string
	Tenant       *string
	TenantRole   *string
}

// CreateUser adds a user and returns it.
func (s *Store) CreateUser(ctx context.Context, n NewUser) (auth.User, error) {
	u := auth.User{Email: n.Email, Name: n.Name, PlatformRole: n.PlatformRole, Tenant: n.Tenant, TenantRole: n.TenantRole}
	err := s.DB.QueryRowContext(ctx, `
		INSERT INTO users (email, name, password_hash, platform_role, tenant_id, tenant_role)
		VALUES (lower($1), $2, $3, $4, $5, $6) RETURNING id`,
		n.Email, n.Name, n.PasswordHash, n.PlatformRole, n.Tenant, n.TenantRole).Scan(&u.ID)
	u.Email = lowerASCII(n.Email)
	return u, translate(err)
}

// Credentials returns the user and password hash for a login.
func (s *Store) Credentials(ctx context.Context, email string) (auth.User, string, error) {
	var u auth.User
	var hash string
	var disabled bool
	err := s.DB.QueryRowContext(ctx, `
		SELECT id, email, name, password_hash, platform_role, tenant_id, tenant_role, disabled, mfa_secret IS NOT NULL
		FROM users WHERE email = lower($1)`, email).
		Scan(&u.ID, &u.Email, &u.Name, &hash, &u.PlatformRole, &u.Tenant, &u.TenantRole, &disabled, &u.MFAEnabled)
	if err == nil && disabled {
		return auth.User{}, "", ErrNotFound
	}
	return u, hash, translate(err)
}

// Users lists the users of a tenant.
func (s *Store) Users(ctx context.Context, tenant string) ([]auth.User, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, email, name, platform_role, tenant_id, tenant_role, mfa_secret IS NOT NULL, time_zone
		FROM users WHERE tenant_id = $1 AND NOT disabled ORDER BY email`, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []auth.User{}
	for rows.Next() {
		var u auth.User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.PlatformRole, &u.Tenant, &u.TenantRole, &u.MFAEnabled, &u.TimeZone); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// CountPlatformAdmins counts the active platform administrators.
func (s *Store) CountPlatformAdmins(ctx context.Context) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE platform_role = 'admin' AND NOT disabled`).Scan(&n)
	return n, err
}

func lowerASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

// DeleteTenant removes a tenant and everything it owns, in one transaction:
// sites with their maps (the gateways drop them at their next reload),
// connectors (the relays close their tunnels), people, invitations,
// settings and its activity. Traffic logs in the log store are not touched:
// they expire on their own.
func (s *Store) DeleteTenant(ctx context.Context, id string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT true FROM tenants WHERE id = $1 FOR UPDATE`, id).Scan(&exists); err != nil {
		return translate(err)
	}
	apps := `(SELECT id FROM apps WHERE tenant_id = $1)`
	users := `(SELECT id FROM users WHERE tenant_id = $1)`
	for _, q := range []string{
		`DELETE FROM unmapped_hits WHERE app_id IN ` + apps,
		`DELETE FROM map_reviews WHERE app_id IN ` + apps,
		`DELETE FROM map_verifications WHERE app_id IN ` + apps,
		`DELETE FROM discoveries WHERE app_id IN ` + apps,
		`DELETE FROM map_versions WHERE app_id IN ` + apps,
		`DELETE FROM apps WHERE tenant_id = $1`, // aliases, certificates, login, alerts, security scans: cascade
		`DELETE FROM connectors WHERE tenant_id = $1`,
		`DELETE FROM invitations WHERE tenant_id = $1 OR invited_by IN ` + users,
		`DELETE FROM audit WHERE tenant_id = $1 OR user_id IN ` + users,
		`DELETE FROM users WHERE tenant_id = $1`, // sessions, password resets: cascade
		`DELETE FROM tenants WHERE id = $1`,      // AI model, scanner, alert settings: cascade
	} {
		if _, err := tx.ExecContext(ctx, q, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SetTimeZone sets the time zone a person sees times in.
func (s *Store) SetTimeZone(ctx context.Context, user int64, zone string) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE users SET time_zone = $2 WHERE id = $1`, user, zone)
	return affected(res, err)
}
