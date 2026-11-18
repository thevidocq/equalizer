package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// reachError explains why a map cannot reach its legacy application.
type reachError struct{ code, msg string }

func (e *reachError) Error() string { return e.msg }

type reachDoc struct {
	Legacy struct {
		Reach struct {
			Mode        string   `json:"mode"`
			ConnectorID string   `json:"connector_id"`
			Allow       []string `json:"allow"`
			Address     string   `json:"address"`
		} `json:"reach"`
	} `json:"legacy"`
}

// checkReach enforces what the schema cannot know:
//   - a connector must belong to the map's tenant, or a customer could reach
//     another customer's internal network;
//   - direct mode reaches only destinations the platform allows
//     (EQ_DIRECT_DESTINATIONS), or a map could point the gateway at the
//     platform's own services;
//   - reach.address must be one of the allowed destinations.
func (s *Server) checkReach(ctx context.Context, tenant string, doc json.RawMessage) error {
	var d reachDoc
	if err := json.Unmarshal(doc, &d); err != nil {
		return &reachError{"invalid_map", "unreadable legacy.reach"}
	}
	rc := d.Legacy.Reach
	if rc.Address != "" && !containsDest(rc.Allow, rc.Address) {
		return &reachError{"invalid_map", "legacy.reach.address must be in legacy.reach.allow"}
	}
	switch rc.Mode {
	case "connector":
		owner, err := s.Store.ConnectorTenant(ctx, rc.ConnectorID)
		if errors.Is(err, store.ErrNotFound) || (err == nil && owner != tenant) {
			// Same answer whether it exists elsewhere or not at all.
			return &reachError{"connector_not_found", fmt.Sprintf("no connector %q in this tenant", rc.ConnectorID)}
		}
		return err
	case "direct":
		for _, a := range rc.Allow {
			if !s.DirectDestinations[normDest(a)] {
				return &reachError{"direct_not_allowed",
					"direct mode is limited to destinations allowed by the platform; use a connector"}
			}
		}
	}
	return nil
}

func containsDest(list []string, d string) bool {
	for _, a := range list {
		if normDest(a) == normDest(d) {
			return true
		}
	}
	return false
}

func normDest(d string) string {
	host, port, err := net.SplitHostPort(strings.TrimSpace(d))
	if err != nil {
		return ""
	}
	return net.JoinHostPort(strings.ToLower(host), port)
}

// ParseDestinations reads EQ_DIRECT_DESTINATIONS (host:port, comma separated).
func ParseDestinations(raw string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, d := range strings.Split(raw, ",") {
		if strings.TrimSpace(d) == "" {
			continue
		}
		n := normDest(d)
		if n == "" {
			return nil, fmt.Errorf("invalid destination %q (host:port)", d)
		}
		out[n] = true
	}
	return out, nil
}

func (s *Server) reachFail(w http.ResponseWriter, r *http.Request, err error, status int) {
	var re *reachError
	if errors.As(err, &re) {
		fail(w, status, re.code, re.msg)
		return
	}
	s.storeError(w, r, err)
}
