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
	if m.Meta.Scheme() == "http" {
		return nil, fmt.Errorf("roles.auth: the login with the identity provider needs the site in https (meta.public_scheme)")
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
		return nil, fmt.Errorf("roles.auth: login not required everywhere needs roles.anonymous")
	}
	for _, p := range a.Paths {
		if !strings.HasPrefix(p, "/") {
			return nil, fmt.Errorf("roles.auth.paths: %q must start with /", p)
		}
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

// checkEntryRoles refuses entries open to a role the map does not declare:
// a typo would silently close the page to everyone.
func checkEntryRoles(m *mapmodel.Model) error {
	check := func(kind, id string, roles []string) error {
		for _, r := range roles {
			if _, ok := m.Roles.List[r]; !ok {
				return fmt.Errorf("%s %q: role %q is not in roles.list", kind, id, r)
			}
		}
		return nil
	}
	for _, r := range m.Routes {
		if err := check("route", r.ID, r.Roles); err != nil {
			return err
		}
	}
	for _, f := range m.Forms {
		if err := check("form", f.ID, f.Roles); err != nil {
			return err
		}
	}
	for _, a := range m.API {
		if err := check("api", a.ID, a.Roles); err != nil {
			return err
		}
	}
	for _, a := range m.Assets {
		if err := check("assets", a.Prefix, a.Roles); err != nil {
			return err
		}
	}
	return nil
}
