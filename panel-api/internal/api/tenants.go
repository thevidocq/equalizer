package api

import (
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

type tenantCreate struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (s *Server) createTenant(w http.ResponseWriter, r *http.Request, u *auth.User) {
	if !u.PlatformAdmin() {
		forbidden(w)
		return
	}
	var in tenantCreate
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if !slugRe.MatchString(in.ID) || in.Name == "" || len(in.Name) > 200 {
		fail(w, http.StatusUnprocessableEntity, "invalid", "id: lower case letters, digits and -; name: 1 to 200 characters")
		return
	}
	t, err := s.Store.CreateTenant(r.Context(), in.ID, in.Name)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &t.ID, "tenant.create", t.ID, nil)
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request, u *auth.User) {
	tenant := r.PathValue("tenant")
	if !u.Platform() && !u.HasTenantRole(tenant, "admin") {
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

type userCreate struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

var tenantRoles = map[string]bool{"admin": true, "editor": true, "approver": true, "auditor": true}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request, u *auth.User) {
	tenant := r.PathValue("tenant")
	if !u.HasTenantRole(tenant, "admin") {
		forbidden(w)
		return
	}
	var in userCreate
	if !decode(w, r, &in) {
		return
	}
	in.Email, in.Name = strings.TrimSpace(in.Email), strings.TrimSpace(in.Name)
	if !validEmail(in.Email) || in.Name == "" || len(in.Name) > 200 || !tenantRoles[in.Role] {
		fail(w, http.StatusUnprocessableEntity, "invalid", "email, name and role (admin, editor, approver, auditor) are required")
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		fail(w, http.StatusUnprocessableEntity, "weak_password", err.Error())
		return
	}
	created, err := s.Store.CreateUser(r.Context(), store.NewUser{Email: in.Email, Name: in.Name, PasswordHash: hash, Tenant: &tenant, TenantRole: &in.Role})
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &tenant, "user.create", created.Email, map[string]any{"role": in.Role})
	writeJSON(w, http.StatusCreated, created)
}

func validEmail(e string) bool {
	at := strings.LastIndexByte(e, '@')
	return len(e) <= 254 && at > 0 && at < len(e)-3 && strings.Contains(e[at:], ".") && !strings.ContainsAny(e, " \t\r\n<>\"")
}
