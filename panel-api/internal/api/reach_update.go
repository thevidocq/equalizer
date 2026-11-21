package api

import (
	"net/http"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/signing"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// putReach changes how Equalizer reaches a site that already has a published
// map, and republishes that map with the new legacy.reach — the tuned
// entries are kept. The first reach is still chosen when discovery runs the
// first time; afterwards it is changed here (Site → Isolation), not by
// re-running discovery.
func (s *Server) putReach(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	if !u.HasTenantRole(a.Tenant, "admin") {
		forbidden(w)
		return
	}
	var in struct {
		Mode        string `json:"mode"`
		ConnectorID string `json:"connector_id"`
		Address     string `json:"address"`
	}
	if !decode(w, r, &in) {
		return
	}
	reach, err := newReach(in.Mode, in.ConnectorID, in.Address)
	if err != nil {
		writeReachError(w, err)
		return
	}
	v, err := s.Store.Published(r.Context(), a.ID)
	if err == store.ErrNotFound {
		fail(w, http.StatusConflict, "no_published_map", "publish a map first; the first reach is chosen when discovery runs")
		return
	}
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	doc, err := store.SetReach(v.Document, reach)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	if err := s.checkReach(r.Context(), a.Tenant, doc); err != nil {
		writeReachError(w, err)
		return
	}
	if err := s.mapChecked(r.Context(), doc, false); err != nil {
		s.storeError(w, r, err)
		return
	}
	signed, err := signing.Sign(doc, v.Version, s.SigningKey, s.KeyID)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	if err := s.Store.Republish(r.Context(), a.ID, v.Version, doc, signed.Payload, signed.Signature, signed.KeyID); err != nil {
		s.storeError(w, r, err)
		return
	}
	var mode = in.Mode
	if mode == "" {
		mode = "connector"
	}
	_ = s.Store.Record(r.Context(), &u.ID, &a.Tenant, "app.reach", a.ID,
		map[string]any{"mode": mode, "connector_id": in.ConnectorID, "address": in.Address})
	s.writeApp(w, r, a.ID)
}

// writeReachError turns a reachError into the right status.
func writeReachError(w http.ResponseWriter, err error) {
	if re, ok := err.(*reachError); ok {
		code := http.StatusUnprocessableEntity
		if re.code == "connector_not_found" {
			code = http.StatusConflict
		}
		fail(w, code, re.code, re.msg)
		return
	}
	fail(w, http.StatusUnprocessableEntity, "invalid", err.Error())
}
