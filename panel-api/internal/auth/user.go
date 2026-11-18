package auth

// User is a panel user with the roles that decide what they may do.
type User struct {
	ID           int64   `json:"id"`
	Email        string  `json:"email"`
	Name         string  `json:"name"`
	PlatformRole *string `json:"platform_role"`
	Tenant       *string `json:"tenant"`
	TenantRole   *string `json:"tenant_role"`
	MFAEnabled   bool    `json:"mfa_enabled"`
	MFAVerified  bool    `json:"mfa_verified"` // this session passed the second step
}

// Platform reports a platform user (administrator or operator).
func (u *User) Platform() bool { return u.PlatformRole != nil }

// PlatformAdmin may do everything.
func (u *User) PlatformAdmin() bool { return u.PlatformRole != nil && *u.PlatformRole == "admin" }

// CanRead reports whether the user may read the tenant's data.
func (u *User) CanRead(tenant string) bool {
	return u.Platform() || (u.Tenant != nil && *u.Tenant == tenant)
}

// HasTenantRole reports whether the user has one of the roles in the tenant.
// Platform administrators have every role; operators none (read-only).
func (u *User) HasTenantRole(tenant string, roles ...string) bool {
	if u.PlatformAdmin() {
		return true
	}
	if u.Tenant == nil || *u.Tenant != tenant || u.TenantRole == nil {
		return false
	}
	for _, r := range roles {
		if *u.TenantRole == r {
			return true
		}
	}
	return false
}
