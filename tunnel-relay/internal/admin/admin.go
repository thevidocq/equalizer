// Package admin serves /healthz, /readyz and the list of open tunnels on the
// control network.
package admin

import (
	"encoding/json"
	"net/http"
	"sync/atomic"

	"github.com/thevidocq/equalizer/tunnel-relay/internal/sessions"
)

// Handler is the admin endpoint.
type Handler struct {
	Pool    *sessions.Pool
	Ready   *atomic.Bool // true once the connectors file has been loaded
	Version string
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/healthz":
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "version": h.Version})
	case "/readyz":
		if !h.Ready.Load() {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "not_ready", "reason": "connectors_not_loaded"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ready"})
	case "/v1/tunnels":
		writeJSON(w, http.StatusOK, map[string]any{"tunnels": h.Pool.Snapshot()})
	default:
		http.NotFound(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
