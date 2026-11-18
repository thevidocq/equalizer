package api

import (
	"net/http"
	"strconv"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
)

type passwordChange struct {
	Current string `json:"current_password"`
	New     string `json:"new_password"`
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request, u *auth.User) {
	var in passwordChange
	if !decode(w, r, &in) {
		return
	}
	st, err := s.Store.MFA(r.Context(), u.ID)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	if !auth.CheckPassword(st.PassHash, in.Current) {
		fail(w, http.StatusBadRequest, "wrong_password", "the current password is wrong")
		return
	}
	hash, err := auth.HashPassword(in.New)
	if err != nil {
		fail(w, http.StatusBadRequest, "weak_password", err.Error())
		return
	}
	if err := s.Store.SetPassword(r.Context(), u.ID, hash); err != nil {
		s.storeError(w, r, err)
		return
	}
	// Other browsers logged in with the old password are logged out.
	if c, err := r.Cookie(auth.CookieName); err == nil {
		_ = s.Sessions.EndAll(r.Context(), u.ID, c.Value)
	}
	_ = s.Store.Record(r.Context(), &u.ID, u.Tenant, "auth.password_changed", u.Email, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deactivateUser(w http.ResponseWriter, r *http.Request, u *auth.User) {
	tenant := r.PathValue("tenant")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || !u.HasTenantRole(tenant, "admin") {
		forbidden(w)
		return
	}
	if id == u.ID {
		fail(w, http.StatusConflict, "cannot_deactivate_self", "ask another administrator")
		return
	}
	email, err := s.Store.Deactivate(r.Context(), tenant, id)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &tenant, "user.deactivate", email, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) resetMFA(w http.ResponseWriter, r *http.Request, u *auth.User) {
	tenant := r.PathValue("tenant")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || !u.HasTenantRole(tenant, "admin") {
		forbidden(w)
		return
	}
	if err := s.Store.ResetMFA(r.Context(), tenant, id); err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.EndSessions(r.Context(), id)
	_ = s.Store.Record(r.Context(), &u.ID, &tenant, "user.mfa_reset", strconv.FormatInt(id, 10), nil)
	w.WriteHeader(http.StatusNoContent)
}
