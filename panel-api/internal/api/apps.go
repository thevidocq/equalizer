package api

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

var (
	domainRe      = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)
	connectorIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,62}$`)
)

func (s *Server) listApps(w http.ResponseWriter, r *http.Request, u *auth.User) {
	tenant := r.PathValue("tenant")
	if !u.CanRead(tenant) {
		forbidden(w)
		return
	}
	apps, err := s.Store.Apps(r.Context(), tenant)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list(apps))
}

type appCreate struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	PublicDomain string  `json:"public_domain"`
	Kind         *string `json:"kind"`
}

var siteKinds = map[string]bool{"wordpress": true, "joomla": true, "drupal": true, "moodle": true, "generic": true}

func (s *Server) createApp(w http.ResponseWriter, r *http.Request, u *auth.User) {
	tenant := r.PathValue("tenant")
	if !u.HasTenantRole(tenant, "admin") {
		forbidden(w)
		return
	}
	var in appCreate
	if !decode(w, r, &in) {
		return
	}
	in.Name, in.PublicDomain = strings.TrimSpace(in.Name), strings.ToLower(strings.TrimSpace(in.PublicDomain))
	if !slugRe.MatchString(in.ID) || in.Name == "" || len(in.Name) > 200 || !domainRe.MatchString(in.PublicDomain) {
		fail(w, http.StatusUnprocessableEntity, "invalid", "id (slug), name and a valid public_domain are required")
		return
	}
	if in.Kind != nil && !siteKinds[*in.Kind] {
		fail(w, http.StatusUnprocessableEntity, "invalid", "kind: wordpress, joomla, drupal, moodle or generic")
		return
	}
	a, err := s.Store.CreateApp(r.Context(), store.App{ID: in.ID, Tenant: tenant, Name: in.Name, PublicDomain: in.PublicDomain, Kind: in.Kind})
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &tenant, "app.create", a.ID, map[string]any{"public_domain": a.PublicDomain})
	writeJSON(w, http.StatusCreated, a)
}

// app loads the app of the path and checks that the user may read it.
func (s *Server) app(w http.ResponseWriter, r *http.Request, u *auth.User) (store.App, bool) {
	a, err := s.Store.App(r.Context(), r.PathValue("app"))
	if err != nil {
		s.storeError(w, r, err)
		return a, false
	}
	if !u.CanRead(a.Tenant) {
		// Same answer as a missing app: do not reveal other tenants' sites.
		fail(w, http.StatusNotFound, "not_found", "")
		return a, false
	}
	return a, true
}

func (s *Server) getApp(w http.ResponseWriter, r *http.Request, u *auth.User) {
	if a, ok := s.app(w, r, u); ok {
		writeJSON(w, http.StatusOK, a)
	}
}
