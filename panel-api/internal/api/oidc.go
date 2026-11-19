package api

import (
	"net/http"
	"strings"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
)

// OIDC login in front of a site: issuer, client and group → role rules are
// in the map (reviewed and approved like the rest); the client secret is
// kept here, sealed, and given only to the gateways.

func oidcLabel(app string) string { return "oidc:" + app }

const oidcCallback = "/.eq/oidc/callback"

func (s *Server) getOIDC(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	has, err := s.Store.HasOIDCSecret(r.Context(), a.ID)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"redirect_uri": "https://" + a.PublicDomain + oidcCallback, "has_secret": has,
		"logout_uri": "https://" + a.PublicDomain + "/",
	})
}

func (s *Server) putOIDCSecret(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	if !u.HasTenantRole(a.Tenant, "admin") {
		forbidden(w)
		return
	}
	var in struct {
		ClientSecret string `json:"client_secret"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.ClientSecret = strings.TrimSpace(in.ClientSecret)
	if in.ClientSecret == "" || len(in.ClientSecret) > 1000 {
		fail(w, http.StatusUnprocessableEntity, "invalid", "client_secret: 1 to 1000 characters")
		return
	}
	if err := s.Store.SetOIDCSecret(r.Context(), a.ID, s.MFA.SealFor(oidcLabel(a.ID), []byte(in.ClientSecret))); err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &a.Tenant, "oidc.secret_set", a.ID, nil) // never the secret
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteOIDCSecret(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	if !u.HasTenantRole(a.Tenant, "admin") {
		forbidden(w)
		return
	}
	if err := s.Store.DeleteOIDCSecret(r.Context(), a.ID); err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &a.Tenant, "oidc.secret_deleted", a.ID, nil)
	w.WriteHeader(http.StatusNoContent)
}

// internalOIDC gives the gateways the client secrets in clear, on the
// control network with the internal token.
func (s *Server) internalOIDC(w http.ResponseWriter, r *http.Request) {
	all, err := s.Store.OIDCSecrets(r.Context())
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	type item struct {
		App          string `json:"app"`
		ClientSecret string `json:"client_secret"`
	}
	out := make([]item, 0, len(all))
	for _, o := range all {
		secret, err := s.MFA.OpenFor(oidcLabel(o.App), o.Sealed)
		if err != nil {
			s.Log.ErrorContext(r.Context(), "OIDC secret cannot be opened", "app", o.App)
			continue
		}
		out = append(out, item{App: o.App, ClientSecret: string(secret)})
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}
