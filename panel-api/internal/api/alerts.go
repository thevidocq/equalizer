package api

import (
	"net/http"
	"strings"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

type alertSettings struct {
	store.AlertSettings
	MailAvailable bool `json:"mail_available"`
}

func (s *Server) getAlerts(w http.ResponseWriter, r *http.Request, u *auth.User) {
	tenant := r.PathValue("tenant")
	if !u.HasTenantRole(tenant, "admin") {
		forbidden(w)
		return
	}
	a, err := s.Store.Alerts(r.Context(), tenant)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, alertSettings{a, s.MailAvailable})
}

func (s *Server) putAlerts(w http.ResponseWriter, r *http.Request, u *auth.User) {
	tenant := r.PathValue("tenant")
	if !u.HasTenantRole(tenant, "admin") {
		forbidden(w)
		return
	}
	var in store.AlertSettings
	if !decode(w, r, &in) {
		return
	}
	if len(in.ExtraEmails) > 5 {
		fail(w, http.StatusUnprocessableEntity, "invalid", "at most 5 more addresses")
		return
	}
	clean := []string{}
	for _, e := range in.ExtraEmails {
		e = strings.ToLower(strings.TrimSpace(e))
		if !validEmail(e) {
			fail(w, http.StatusUnprocessableEntity, "invalid", "not an email address: "+e)
			return
		}
		clean = append(clean, e)
	}
	in.ExtraEmails = clean
	if err := s.Store.SetAlerts(r.Context(), tenant, in); err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &tenant, "settings.alerts", "", map[string]any{"enabled": in.Enabled, "extra": len(clean)})
	writeJSON(w, http.StatusOK, alertSettings{in, s.MailAvailable})
}
