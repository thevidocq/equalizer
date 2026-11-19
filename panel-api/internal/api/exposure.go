package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/exposure"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// patchApp changes site settings; today only the old public IP.
func (s *Server) patchApp(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	if !u.HasTenantRole(a.Tenant, "admin") {
		forbidden(w)
		return
	}
	var in struct {
		LegacyIP json.RawMessage `json:"legacy_ip"`
		Schedule *string         `json:"discovery_schedule"`
		Scheme   *string         `json:"public_scheme"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.LegacyIP == nil && in.Schedule == nil && in.Scheme == nil {
		fail(w, http.StatusUnprocessableEntity, "invalid", "nothing to change")
		return
	}
	if in.Scheme != nil && *in.Scheme != a.TLS.PublicScheme {
		if err := s.setPublicScheme(r, a, *in.Scheme); err != nil {
			var ie *inputError
			switch {
			case errors.As(err, &ie):
				fail(w, http.StatusUnprocessableEntity, "invalid", ie.msg)
			case !s.mapCheckFail(w, err):
				s.storeError(w, r, err)
			}
			return
		}
		_ = s.Store.Record(r.Context(), &u.ID, &a.Tenant, "app.public_scheme", a.ID, map[string]any{"scheme": *in.Scheme})
		if in.LegacyIP == nil && in.Schedule == nil {
			s.writeApp(w, r, a.ID)
			return
		}
	}
	if in.Schedule != nil {
		if *in.Schedule != "off" && *in.Schedule != "daily" && *in.Schedule != "weekly" {
			fail(w, http.StatusUnprocessableEntity, "invalid", "discovery_schedule: off, daily or weekly")
			return
		}
		if err := s.Store.SetDiscoverySchedule(r.Context(), a.ID, *in.Schedule, u.ID); err != nil {
			s.storeError(w, r, err)
			return
		}
		_ = s.Store.Record(r.Context(), &u.ID, &a.Tenant, "discovery.schedule", a.ID, map[string]any{"schedule": *in.Schedule})
		if in.LegacyIP == nil {
			s.writeApp(w, r, a.ID)
			return
		}
	}
	var ip *string
	if string(in.LegacyIP) != "null" {
		var raw string
		addr, valid := netipFromJSON(in.LegacyIP, &raw)
		if !valid {
			fail(w, http.StatusUnprocessableEntity, "invalid",
				"legacy_ip: a public IPv4 or IPv6 address (private, loopback and reserved addresses are refused)")
			return
		}
		ip = &addr
	}
	if err := s.Store.SetLegacyIP(r.Context(), a.ID, ip); err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &a.Tenant, "app.legacy_ip", a.ID, map[string]any{"set": ip != nil})
	s.writeApp(w, r, a.ID)
}

func netipFromJSON(raw json.RawMessage, into *string) (string, bool) {
	if json.Unmarshal(raw, into) != nil {
		return "", false
	}
	addr, ok := exposure.PublicIP(*into)
	if !ok {
		return "", false
	}
	return addr.String(), true
}

// checkExposure runs the check now, at most once a minute per site.
func (s *Server) checkExposure(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	if !u.HasTenantRole(a.Tenant, "admin") {
		forbidden(w)
		return
	}
	if a.Exposure.LegacyIP == nil || s.Exposure == nil {
		fail(w, http.StatusConflict, "no_legacy_ip", "set the site's old public IP first")
		return
	}
	key := "exposure:" + a.ID
	if s.ExposureThrottle.Blocked(key) {
		fail(w, http.StatusTooManyRequests, "too_many_checks", "one check a minute per site")
		return
	}
	s.ExposureThrottle.Fail(key)
	s.Exposure.CheckOne(r.Context(), store.ExposureTarget{
		App: a.ID, Tenant: a.Tenant, Domain: a.PublicDomain, IP: *a.Exposure.LegacyIP, Status: a.Exposure.Status,
	})
	s.writeApp(w, r, a.ID)
}

func (s *Server) writeApp(w http.ResponseWriter, r *http.Request, id string) {
	a, err := s.Store.App(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		fail(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}
