package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

func (s *Server) listTenants(w http.ResponseWriter, r *http.Request, u *auth.User) {
	var only *string
	if !u.Platform() {
		only = u.Tenant
	}
	ts, err := s.Store.Tenants(r.Context(), only)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list(ts))
}

// platformTenants is what people who run the service see: tenants, their
// sites' names and status. Nothing of their configuration.
func (s *Server) platformTenants(w http.ResponseWriter, r *http.Request, u *auth.User) {
	if !u.Platform() {
		forbidden(w)
		return
	}
	ov, err := s.Store.Overview(r.Context())
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list(ov))
}

type tenantCreate struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	AdminEmail string `json:"admin_email"`
	AdminName  string `json:"admin_name"`
}

// createTenant adds a tenant and invites its first administrator, who
// chooses their own password: the platform never knows it.
func (s *Server) createTenant(w http.ResponseWriter, r *http.Request, u *auth.User) {
	if !u.PlatformAdmin() {
		forbidden(w)
		return
	}
	var in tenantCreate
	if !decode(w, r, &in) {
		return
	}
	in.Name, in.AdminName, in.AdminEmail = strings.TrimSpace(in.Name), strings.TrimSpace(in.AdminName), strings.TrimSpace(in.AdminEmail)
	if !slugRe.MatchString(in.ID) || in.Name == "" || len(in.Name) > 200 || !validEmail(in.AdminEmail) || in.AdminName == "" || len(in.AdminName) > 200 {
		fail(w, http.StatusUnprocessableEntity, "invalid", "id (lower case letters, digits, -), name, administrator's name and email are required")
		return
	}
	if !s.emailFree(w, r, in.AdminEmail, "") {
		return
	}
	t, err := s.Store.CreateTenant(r.Context(), in.ID, in.Name)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	inv, err := s.invite(r, u, store.Invitation{Tenant: t.ID, TenantName: t.Name, Email: in.AdminEmail, Name: in.AdminName, Role: "admin"})
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	// A platform action: recorded in the platform's activity, not the tenant's.
	_ = s.Store.Record(r.Context(), &u.ID, nil, "tenant.create", t.ID, map[string]any{"invitation_sent": inv.Sent})
	writeJSON(w, http.StatusCreated, map[string]any{"tenant": t, "invitation": inv})
}

type firstAdmin struct {
	Email string `json:"admin_email"`
	Name  string `json:"admin_name"`
}

// reinviteFirstAdmin replaces the invitation of a tenant's first
// administrator (lost or unsent email, wrong address). Refused with 409 once
// anyone has joined: from then on only the tenant invites people.
func (s *Server) reinviteFirstAdmin(w http.ResponseWriter, r *http.Request, u *auth.User) {
	if !u.PlatformAdmin() {
		forbidden(w)
		return
	}
	var in firstAdmin
	if !decode(w, r, &in) {
		return
	}
	in.Email, in.Name = strings.TrimSpace(in.Email), strings.TrimSpace(in.Name)
	if !validEmail(in.Email) || in.Name == "" || len(in.Name) > 200 {
		fail(w, http.StatusUnprocessableEntity, "invalid", "administrator's name and email are required")
		return
	}
	if !s.emailFree(w, r, in.Email, r.PathValue("tenant")) {
		return
	}
	t, err := s.Store.RestartTenant(r.Context(), r.PathValue("tenant"))
	if errors.Is(err, store.ErrState) {
		fail(w, http.StatusConflict, "tenant_in_use", "people already joined this tenant: its administrators invite the others")
		return
	}
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	inv, err := s.invite(r, u, store.Invitation{Tenant: t.ID, TenantName: t.Name, Email: in.Email, Name: in.Name, Role: "admin"})
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, nil, "tenant.reinvite", t.ID, map[string]any{"invitation_sent": inv.Sent})
	writeJSON(w, http.StatusCreated, inv)
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request, u *auth.User) {
	tenant := r.PathValue("tenant")
	if !u.HasTenantRole(tenant, "admin") {
		forbidden(w)
		return
	}
	users, err := s.Store.Users(r.Context(), tenant)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list(users))
}

func validEmail(e string) bool {
	at := strings.LastIndexByte(e, '@')
	return len(e) <= 254 && at > 0 && at < len(e)-3 && strings.Contains(e[at:], ".") && !strings.ContainsAny(e, " \t\r\n<>\"")
}
