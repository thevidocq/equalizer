package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/health"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// Connectors belong to the tenant: one per server, shared by its sites.

type connectorCreated struct {
	ID     string `json:"id"`
	Tenant string `json:"tenant"`
	Token  string `json:"token"`
}

func newToken() (string, string) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	tok := base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(tok))
	return tok, hex.EncodeToString(sum[:])
}

// tenantAdmin returns the tenant of the path when the user administers it.
func tenantAdmin(w http.ResponseWriter, r *http.Request, u *auth.User) (string, bool) {
	tenant := r.PathValue("tenant")
	if !u.HasTenantRole(tenant, "admin") {
		forbidden(w)
		return "", false
	}
	return tenant, true
}

func (s *Server) listConnectors(w http.ResponseWriter, r *http.Request, u *auth.User) {
	tenant, ok := tenantAdmin(w, r, u)
	if !ok {
		return
	}
	cs, err := s.Store.Connectors(r.Context(), tenant)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	var tunnels map[string][]string
	if s.Health != nil {
		tunnels, _ = s.Health.TunnelVersions(r.Context())
	}
	for i := range cs {
		cs[i].Versions = tunnels[cs[i].ID]
		if cs[i].Versions == nil {
			cs[i].Versions = []string{}
		}
		cs[i].Online = len(cs[i].Versions) > 0
		for _, v := range cs[i].Versions {
			cs[i].Outdated = cs[i].Outdated || health.Older(v, s.Version)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": cs, "next_cursor": nil, "platform_version": s.Version, "relay_url": s.RelayURL})
}

func (s *Server) createConnector(w http.ResponseWriter, r *http.Request, u *auth.User) {
	tenant, ok := tenantAdmin(w, r, u)
	if !ok {
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
	if _, err := s.Store.CreateConnector(r.Context(), tenant, in.ID, hash); err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &tenant, "connector.create", in.ID, nil)
	writeJSON(w, http.StatusCreated, connectorCreated{ID: in.ID, Tenant: tenant, Token: tok})
}

func (s *Server) rotateConnector(w http.ResponseWriter, r *http.Request, u *auth.User) {
	tenant, ok := tenantAdmin(w, r, u)
	if !ok {
		return
	}
	id := r.PathValue("id")
	tok, hash := newToken()
	if err := s.Store.RotateConnector(r.Context(), tenant, id, hash); err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &tenant, "connector.rotate", id, nil)
	writeJSON(w, http.StatusOK, connectorCreated{ID: id, Tenant: tenant, Token: tok})
}

func (s *Server) deleteConnector(w http.ResponseWriter, r *http.Request, u *auth.User) {
	tenant, ok := tenantAdmin(w, r, u)
	if !ok {
		return
	}
	id := r.PathValue("id")
	err := s.Store.DeleteConnector(r.Context(), tenant, id)
	if errors.Is(err, store.ErrState) {
		fail(w, http.StatusConflict, "connector_in_use", "a published map uses this connector: publish a map without it first")
		return
	}
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &tenant, "connector.delete", id, nil)
	w.WriteHeader(http.StatusNoContent)
}
