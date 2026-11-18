package store

import (
	"context"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
)

// Invitation is a pending invitation (the token is stored as SHA-256 only).
type Invitation struct {
	ID         int64     `json:"id"`
	Tenant     string    `json:"-"`
	TenantName string    `json:"-"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	Role       string    `json:"role"`
	ExpiresAt  time.Time `json:"expires_at"`
	Sent       bool      `json:"sent"`
}

// CreateInvitation stores an invitation.
func (s *Store) CreateInvitation(ctx context.Context, inv Invitation, tokenHash string, invitedBy int64) (Invitation, error) {
	err := s.DB.QueryRowContext(ctx, `
		INSERT INTO invitations (tenant_id, email, name, role, token_sha256, invited_by, expires_at)
		VALUES ($1, lower($2), $3, $4, $5, $6, $7) RETURNING id, email`,
		inv.Tenant, inv.Email, inv.Name, inv.Role, tokenHash, invitedBy, inv.ExpiresAt).Scan(&inv.ID, &inv.Email)
	return inv, translate(err)
}

// MarkSent records that the invitation email left.
func (s *Store) MarkSent(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE invitations SET sent = true WHERE id = $1`, id)
	return err
}

// Invitations lists the pending invitations of a tenant.
func (s *Store) Invitations(ctx context.Context, tenant string) ([]Invitation, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, tenant_id, email, name, role, expires_at, sent FROM invitations
		WHERE tenant_id = $1 AND accepted_at IS NULL AND NOT revoked AND expires_at > now() ORDER BY created_at DESC`, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Invitation{}
	for rows.Next() {
		var i Invitation
		if err := rows.Scan(&i.ID, &i.Tenant, &i.Email, &i.Name, &i.Role, &i.ExpiresAt, &i.Sent); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// RevokeInvitation revokes a pending invitation of a tenant.
func (s *Store) RevokeInvitation(ctx context.Context, tenant string, id int64) error {
	res, err := s.DB.ExecContext(ctx, `
		UPDATE invitations SET revoked = true WHERE id = $1 AND tenant_id = $2 AND accepted_at IS NULL AND NOT revoked`, id, tenant)
	return affected(res, err)
}

// InvitationByToken returns a usable invitation.
func (s *Store) InvitationByToken(ctx context.Context, tokenHash string) (Invitation, error) {
	var i Invitation
	err := s.DB.QueryRowContext(ctx, `
		SELECT i.id, i.tenant_id, t.name, i.email, i.name, i.role, i.expires_at, i.sent
		FROM invitations i JOIN tenants t ON t.id = i.tenant_id
		WHERE i.token_sha256 = $1 AND i.accepted_at IS NULL AND NOT i.revoked AND i.expires_at > now()`, tokenHash).
		Scan(&i.ID, &i.Tenant, &i.TenantName, &i.Email, &i.Name, &i.Role, &i.ExpiresAt, &i.Sent)
	return i, translate(err)
}

// AcceptInvitation creates the user and consumes the invitation in one
// transaction: an invitation is used once.
func (s *Store) AcceptInvitation(ctx context.Context, tokenHash, passwordHash string) (auth.User, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return auth.User{}, err
	}
	defer tx.Rollback() //nolint:errcheck
	var inv Invitation
	err = tx.QueryRowContext(ctx, `
		UPDATE invitations SET accepted_at = now()
		WHERE token_sha256 = $1 AND accepted_at IS NULL AND NOT revoked AND expires_at > now()
		RETURNING tenant_id, email, name, role`, tokenHash).Scan(&inv.Tenant, &inv.Email, &inv.Name, &inv.Role)
	if err != nil {
		return auth.User{}, translate(err)
	}
	u := auth.User{Email: inv.Email, Name: inv.Name, Tenant: &inv.Tenant, TenantRole: &inv.Role}
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO users (email, name, password_hash, tenant_id, tenant_role) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		inv.Email, inv.Name, passwordHash, inv.Tenant, inv.Role).Scan(&u.ID); err != nil {
		return auth.User{}, translate(err)
	}
	return u, tx.Commit()
}

// TenantOverview is the platform's view of a tenant: names and status only.
type TenantOverview struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	CreatedAt time.Time     `json:"created_at"`
	Joined    bool          `json:"joined"` // someone accepted an invitation
	SiteCount int           `json:"site_count"`
	Sites     []SiteSummary `json:"sites"`
}

// SiteSummary is a site as the platform sees it.
type SiteSummary struct {
	Name   string `json:"name"`
	Status string `json:"status"` // protected, setting_up
}

// Overview lists every tenant with its sites' names and status.
func (s *Store) Overview(ctx context.Context) ([]TenantOverview, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT t.id, t.name, t.created_at, EXISTS (SELECT 1 FROM users u WHERE u.tenant_id = t.id), a.name,
			EXISTS (SELECT 1 FROM map_versions m WHERE m.app_id = a.id AND m.status = 'published'),
			a.exposure_status = 'exposed'
		FROM tenants t LEFT JOIN apps a ON a.tenant_id = t.id
		ORDER BY t.name, t.id, a.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TenantOverview{}
	for rows.Next() {
		var t TenantOverview
		var site *string
		var published, exposed *bool
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt, &t.Joined, &site, &published, &exposed); err != nil {
			return nil, err
		}
		if len(out) == 0 || out[len(out)-1].ID != t.ID {
			t.Sites = []SiteSummary{}
			out = append(out, t)
		}
		if site != nil {
			cur := &out[len(out)-1]
			status := "setting_up"
			if published != nil && *published {
				status = "protected"
			}
			if exposed != nil && *exposed {
				status = "exposed"
			}
			cur.Sites = append(cur.Sites, SiteSummary{Name: *site, Status: status})
			cur.SiteCount++
		}
	}
	return out, rows.Err()
}

// RestartTenant prepares a new invitation for a tenant's first administrator:
// allowed only while nobody has joined the tenant, so the platform never
// gains a way into a tenant that is in use. Pending invitations are revoked.
func (s *Store) RestartTenant(ctx context.Context, tenant string) (Tenant, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Tenant{}, err
	}
	defer tx.Rollback()
	t := Tenant{ID: tenant}
	var joined bool
	err = tx.QueryRowContext(ctx, `
		SELECT name, created_at, EXISTS (SELECT 1 FROM users WHERE tenant_id = $1)
		FROM tenants WHERE id = $1 FOR UPDATE`, tenant).Scan(&t.Name, &t.CreatedAt, &joined)
	if err != nil {
		return Tenant{}, translate(err)
	}
	if joined {
		return Tenant{}, ErrState
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE invitations SET revoked = true WHERE tenant_id = $1 AND accepted_at IS NULL AND NOT revoked`, tenant); err != nil {
		return Tenant{}, err
	}
	return t, tx.Commit()
}
