package api

import (
	"errors"
	"net/http"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/templates"
)

func (s *Server) listTemplates(w http.ResponseWriter, _ *http.Request, _ *auth.User) {
	writeJSON(w, http.StatusOK, list(templates.List()))
}

// mapTemplate fills a template for the site. Nothing is saved: the editor
// receives the document, and it becomes a draft with the usual checks.
func (s *Server) mapTemplate(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	if !u.HasTenantRole(a.Tenant, "admin", "editor") {
		forbidden(w)
		return
	}
	var in struct {
		Template    string `json:"template"`
		ConnectorID string `json:"connector_id"`
		Address     string `json:"address"`
		Scheme      string `json:"scheme"`
		MaxUploadMB int    `json:"max_upload_mb"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Scheme == "" {
		in.Scheme = "https"
	}
	if !connectorIDRe.MatchString(in.ConnectorID) || normDest(in.Address) == "" || (in.Scheme != "https" && in.Scheme != "http") {
		fail(w, http.StatusUnprocessableEntity, "invalid", "connector_id, address (host:port) and scheme (https or http) are required")
		return
	}
	if in.MaxUploadMB < 0 || in.MaxUploadMB > 1024 {
		fail(w, http.StatusUnprocessableEntity, "invalid", "max_upload_mb must be between 1 and 1024")
		return
	}
	doc, err := templates.Render(in.Template, templates.Fill{BaseURL: in.Scheme + "://" + a.PublicDomain,
		Connector: in.ConnectorID, Address: normDest(in.Address), MaxUploadMB: in.MaxUploadMB})
	if errors.Is(err, templates.ErrUnknown) {
		fail(w, http.StatusNotFound, "not_found", "unknown template")
		return
	}
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"document": doc})
}
