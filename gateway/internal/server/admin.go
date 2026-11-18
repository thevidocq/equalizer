package server

import (
	"encoding/json"
	"net/http"

	"github.com/thevidocq/equalizer/gateway/internal/site"
)

// AdminMaps is what the administration endpoints read of the maps.
type AdminMaps interface {
	All() []*site.Site
	Serves(domain string) bool
}

// Admin exposes /healthz, /readyz and /tls/allowed on the internal network
// (never public).
type Admin struct {
	Maps    AdminMaps
	Version string
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
	// Caddy asks here before getting a certificate on demand: only the
	// domains of published maps get one.
	mux.HandleFunc("GET /tls/allowed", func(w http.ResponseWriter, r *http.Request) {
		if a.Maps.Serves(r.URL.Query().Get("domain")) {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
