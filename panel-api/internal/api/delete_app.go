package api

import (
	"net/http"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
)

// deleteApp removes a site and everything that belongs to it: maps,
// connectors, discoveries, reviews, verifications, recorded hits, the
// certificate, OIDC and alerts (the cascading foreign keys do the rest).
// Only a tenant administrator can do it. The gateway stops serving the site
// on its next map fetch, and the connector can no longer reconnect.
func (s *Server) deleteApp(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	if !u.HasTenantRole(a.Tenant, "admin") {
		forbidden(w)
		return
	}
	if err := s.Store.DeleteApp(r.Context(), a.ID); err != nil {
		s.storeError(w, r, err) // ErrNotFound (a concurrent delete) becomes 404
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &a.Tenant, "app.delete", a.ID,
		map[string]any{"name": a.Name, "public_domain": a.PublicDomain})
	w.WriteHeader(http.StatusNoContent)
}
