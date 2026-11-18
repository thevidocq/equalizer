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
		SELECT id, email, name, platform_role, tenant_id, tenant_role, mfa_secret IS NOT NULL
		FROM users WHERE tenant_id = $1 AND NOT disabled ORDER BY email`, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []auth.User{}
	for rows.Next() {
		var u auth.User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.PlatformRole, &u.Tenant, &u.TenantRole, &u.MFAEnabled); err != nil {
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
