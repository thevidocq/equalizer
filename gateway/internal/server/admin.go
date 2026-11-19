package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/site"
)

// AdminMaps is what the administration endpoints read of the maps.
type AdminMaps interface {
	All() []*site.Site
	Serves(domain string) bool
}

// Certificates give the PEM bundle (chain and key) of a domain, or nil.
type Certificates interface {
	Bundle(domain string) []byte
}

// Admin exposes /healthz, /readyz and the TLS endpoints Caddy asks on the
// internal network (never public).
type Admin struct {
	Maps    AdminMaps
	Version string
	Certs   Certificates // nil: no site certificate
}

// Handler returns the handler of the administration endpoints.
func (a *Admin) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "version": a.Version})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		all := a.Maps.All()
		if len(all) == 0 {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{
				"status": "not_ready", "reason": "no valid map loaded",
			})
			return
		}
		sites := make([]map[string]any, 0, len(all))
		for _, s := range all {
			sites = append(sites, map[string]any{"tenant": s.Model.Meta.Tenant, "app": s.Model.Meta.App,
				"domain": s.Model.Meta.PublicDomain, "map_version": s.Model.Meta.Version})
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ready", "sites": sites})
	})
	// The panel asks here whether a draft would load: the same checks as a
	// published map (roles, types, authorisation, issues, login…), so a
	// version the gateway would refuse is never approved or published.
	mux.HandleFunc("POST /maps/check", func(w http.ResponseWriter, r *http.Request) {
		var m mapmodel.Model
		if err := json.NewDecoder(io.LimitReader(r.Body, 8<<20)).Decode(&m); err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"ok": false, "error": "unreadable map: " + err.Error()})
			return
		}
		if _, err := site.Build(&m); err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	// Caddy asks here, at each TLS handshake, for the certificate of a
	// protected site: the legacy site's own, uploaded in the panel. 204: none
	// (http site, or none uploaded yet).
	mux.HandleFunc("GET /tls/certificate", func(w http.ResponseWriter, r *http.Request) {
		name := strings.ToLower(strings.TrimSuffix(r.URL.Query().Get("server_name"), "."))
		var b []byte
		if a.Certs != nil && a.Maps.Serves(name) {
			b = a.Certs.Bundle(name)
		}
		if b == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/x-pem-file")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(b)
	})
	// Caddy asks here before getting a certificate itself (on demand): never
	// for a protected site. Only the platform's own domains (panel, relay)
	// get a Let's Encrypt certificate, and those are not on demand.
	mux.HandleFunc("GET /tls/allowed", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
