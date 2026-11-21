package api

import (
	"context"
	"net/http"
	"sort"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
)

// serviceRelease is a service of the platform and the releases its replicas
// run (more than one while an update rolls out).
type serviceRelease struct {
	Name     string   `json:"name"`
	Releases []string `json:"releases"`
}

// about tells the release of the platform, for the panel's footer; platform
// administrators also get the release of each service.
func (s *Server) about(w http.ResponseWriter, r *http.Request, u *auth.User) {
	out := map[string]any{"release": s.Version}
	if u.Platform() && s.Health != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		services := []serviceRelease{{Name: "Panel", Releases: []string{s.Version}}}
		for _, svc := range []struct{ name, addr string }{{"Gateway", s.Health.GatewayAdmin}, {"Tunnel relay", s.Health.RelayAdmin}} {
			if svc.addr == "" {
				continue
			}
			var rel []string
			for v := range s.Health.Versions(ctx, svc.addr) {
				rel = append(rel, v)
			}
			sort.Strings(rel)
			services = append(services, serviceRelease{Name: svc.name, Releases: rel})
		}
		out["services"] = services
	}
	writeJSON(w, http.StatusOK, out)
}
