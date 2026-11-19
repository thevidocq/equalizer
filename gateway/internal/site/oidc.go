package site

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/oidc"
)

// checkOIDC validates the login with the identity provider, when the map
// turns it on: an https issuer, a client, roles of the map.
func checkOIDC(m *mapmodel.Model) (*oidc.Provider, error) {
	if !m.OIDC() {
		return nil, nil
	}
	a := m.Roles.Auth
	if !strings.HasPrefix(a.Issuer, "https://") || a.ClientID == "" {
		return nil, fmt.Errorf("roles.auth: oidc needs an https issuer and a client_id")
	}
	if len(a.Roles) == 0 {
		return nil, fmt.Errorf("roles.auth: oidc needs at least one group → role")
	}
	for _, r := range a.Roles {
		if _, ok := m.Roles.List[r.Role]; !ok {
			return nil, fmt.Errorf("roles.auth: role %q is not in roles.list", r.Role)
		}
	}
	if !a.LoginRequired() && m.Roles.Anonymous == "" {
		return nil, fmt.Errorf("roles.auth: login not required needs roles.anonymous")
	}
	return &oidc.Provider{Issuer: a.Issuer, Client: &http.Client{Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

// OIDCRules returns the group → role rules of the map.
func (s *Site) OIDCRules() []oidc.RoleRule {
	var out []oidc.RoleRule
	for _, r := range s.Model.Roles.Auth.Roles {
		out = append(out, oidc.RoleRule{Group: r.Group, Role: r.Role})
	}
	return out
}
