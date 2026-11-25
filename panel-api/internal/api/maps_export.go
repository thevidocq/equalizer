package api

import (
	"fmt"
	"net/http"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/mapexport"
)

// exportMap writes a version for another tool (mapexport): OpenAPI 3 or
// ModSecurity rules. Anyone who can read the version can export it.
func (s *Server) exportMap(w http.ResponseWriter, r *http.Request, u *auth.User) {
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
	var (
		out       []byte
		name, ct  string
		exportErr error
	)
	switch r.URL.Query().Get("format") {
	case "openapi":
		out, exportErr = mapexport.OpenAPI(v.Document, a.ID, a.PublicDomain, version)
		name, ct = fmt.Sprintf("%s-map-v%d.openapi.json", a.ID, version), "application/json"
	case "modsecurity":
		out, exportErr = mapexport.ModSecurity(v.Document, a.ID, version)
		name, ct = fmt.Sprintf("%s-map-v%d.modsecurity.conf", a.ID, version), "text/plain; charset=utf-8"
	default:
		fail(w, http.StatusUnprocessableEntity, "invalid", "format: openapi or modsecurity")
		return
	}
	if exportErr != nil {
		fail(w, http.StatusUnprocessableEntity, "not_exported", exportErr.Error())
		return
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(out)
}
