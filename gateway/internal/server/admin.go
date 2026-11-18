package server

import (
	"encoding/json"
	"net/http"
)

// Admin espone /healthz e /readyz sulla rete interna (mai pubblici).
type Admin struct {
	Maps    MapSource
	Version string
}

// Handler restituisce l'handler degli endpoint di amministrazione.
func (a *Admin) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "version": a.Version})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		active := a.Maps.Current()
		if active == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{
				"status": "not_ready", "reason": "nessuna mappa valida caricata",
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":      "ready",
			"tenant":      active.Model.Meta.Tenant,
			"app":         active.Model.Meta.App,
			"map_version": active.Model.Meta.Version,
		})
	})
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
