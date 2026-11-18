package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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
	ID           string `json:"id"`
	Name         string `json:"name"`
	PublicDomain string `json:"public_domain"`
}

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
	a, err := s.Store.CreateApp(r.Context(), store.App{ID: in.ID, Tenant: tenant, Name: in.Name, PublicDomain: in.PublicDomain})
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

func (s *Server) listConnectors(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	cs, err := s.Store.Connectors(r.Context(), a.ID)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list(cs))
}

type connectorCreated struct {
	ID    string `json:"id"`
	App   string `json:"app"`
	Token string `json:"token"`
}

func newToken() (string, string) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	tok := base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(tok))
	return tok, hex.EncodeToString(sum[:])
}

func (s *Server) createConnector(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	if !u.HasTenantRole(a.Tenant, "admin") {
		forbidden(w)
		return
	}
	var in struct {
		ID string `json:"id"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !connectorIDRe.MatchString(in.ID) {
		fail(w, http.StatusUnprocessableEntity, "invalid", "id: lower case letters, digits, - and _")
		return
	}
	tok, hash := newToken()
	if _, err := s.Store.CreateConnector(r.Context(), a.ID, in.ID, hash); err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &a.Tenant, "connector.create", in.ID, map[string]any{"app": a.ID})
	writeJSON(w, http.StatusCreated, connectorCreated{ID: in.ID, App: a.ID, Token: tok})
}

func (s *Server) rotateConnector(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	if !u.HasTenantRole(a.Tenant, "admin") {
		forbidden(w)
		return
	}
	id := r.PathValue("id")
	tok, hash := newToken()
	if err := s.Store.RotateConnector(r.Context(), a.ID, id, hash); err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &a.Tenant, "connector.rotate", id, map[string]any{"app": a.ID})
	writeJSON(w, http.StatusOK, connectorCreated{ID: id, App: a.ID, Token: tok})
}

func (s *Server) deleteConnector(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	if !u.HasTenantRole(a.Tenant, "admin") {
		forbidden(w)
		return
	}
	id := r.PathValue("id")
	if err := s.Store.DeleteConnector(r.Context(), a.ID, id); err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &a.Tenant, "connector.delete", id, map[string]any{"app": a.ID})
	w.WriteHeader(http.StatusNoContent)
}
