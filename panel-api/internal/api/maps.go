package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/mapschema"
	"github.com/thevidocq/equalizer/panel-api/internal/signing"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

func (s *Server) listMaps(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	vs, err := s.Store.Versions(r.Context(), a.ID)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list(vs))
}

func (s *Server) getMap(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	version, ok := pathInt(r, "version")
	if !ok {
		fail(w, http.StatusNotFound, "not_found", "")
		return
	}
	v, err := s.Store.Version(r.Context(), a.ID, version)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

type mapCreate struct {
	Document json.RawMessage `json:"document"`
	Note     string          `json:"note"`
}

func (s *Server) createMap(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	if !u.HasTenantRole(a.Tenant, "admin", "editor") {
		forbidden(w)
		return
	}
	var in mapCreate
	if !decode(w, r, &in) {
		return
	}
	if len(in.Note) > 2000 {
		fail(w, http.StatusUnprocessableEntity, "invalid", "note: at most 2000 characters")
		return
	}
	// The panel owns the identity fields: whatever the document says.
	doc, err := store.SetMeta(in.Document, map[string]any{
		"schema": "gateway-map/1.0", "tenant": a.Tenant, "app": a.ID, "version": 1, "status": "draft",
		"created_by": "user:" + u.Email,
	})
	if err != nil {
		fail(w, http.StatusUnprocessableEntity, "invalid_map", "the document must be a JSON object")
		return
	}
	if err := mapschema.Validate(doc); err != nil {
		var ie *mapschema.ErrInvalid
		if errors.As(err, &ie) {
			fail(w, http.StatusUnprocessableEntity, "invalid_map", "the map does not respect the schema", ie.Details...)
			return
		}
		s.storeError(w, r, err)
		return
	}
	version, err := s.Store.CreateDraft(r.Context(), a.ID, doc, strings.TrimSpace(in.Note), u.ID)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &a.Tenant, "map.draft", a.ID, map[string]any{"version": version})
	writeJSON(w, http.StatusCreated, map[string]any{"app": a.ID, "version": version, "status": "draft"})
}

func (s *Server) mapAction(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	version, ok := pathInt(r, "version")
	if !ok {
		fail(w, http.StatusNotFound, "not_found", "")
		return
	}
	v, err := s.Store.Version(r.Context(), a.ID, version)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	action := r.PathValue("action")
	switch action {
	case "submit":
		if !u.HasTenantRole(a.Tenant, "admin", "editor") {
			forbidden(w)
			return
		}
		err = s.Store.Transition(r.Context(), a.ID, version, []string{"draft"}, "in_review", nil)
	case "approve", "reject":
		if !u.HasTenantRole(a.Tenant, "admin", "approver") {
			forbidden(w)
			return
		}
		if v.AuthorID == u.ID {
			fail(w, http.StatusForbidden, "author_cannot_decide", "a version is approved or rejected by someone other than its author")
			return
		}
		to := map[string]string{"approve": "approved", "reject": "rejected"}[action]
		err = s.Store.Transition(r.Context(), a.ID, version, []string{"in_review"}, to, &u.ID)
	case "publish":
		if !u.HasTenantRole(a.Tenant, "admin", "approver") {
			forbidden(w)
			return
		}
		err = s.publish(r, v)
	default:
		fail(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &a.Tenant, "map."+action, a.ID, map[string]any{"version": version})
	out, err := s.Store.Version(r.Context(), a.ID, version)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	out.Document = nil
	writeJSON(w, http.StatusOK, out)
}

// publish signs the approved document and makes it the published version.
func (s *Server) publish(r *http.Request, v store.MapVersion) error {
	if v.Status != "approved" {
		return store.ErrState
	}
	approver := ""
	if v.Approver != nil {
		approver = "user:" + *v.Approver
	}
	doc, err := store.SetMeta(v.Document, map[string]any{
		"version": v.Version, "status": "published", "approved_by": approver,
		"published_at": time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	signed, err := signing.Sign(doc, v.Version, s.SigningKey, s.KeyID)
	if err != nil {
		return err
	}
	return s.Store.Publish(r.Context(), v.App, v.Version, doc, signed.Payload, signed.Signature, signed.KeyID)
}

func (s *Server) listAudit(w http.ResponseWriter, r *http.Request, u *auth.User) {
	// Platform users read the platform's own activity (tenant nil); tenant
	// people read their tenant's.
	var tenant *string
	switch {
	case u.Platform():
	case u.Tenant != nil && u.HasTenantRole(*u.Tenant, "admin", "approver", "auditor"):
		tenant = u.Tenant
	default:
		forbidden(w)
		return
	}
	entries, err := s.Store.Audit(r.Context(), tenant, 200)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list(entries))
}
