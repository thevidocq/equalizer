package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/thevidocq/equalizer/panel-api/internal/exposure"
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
//   - direct mode reaches public IP addresses written as such (a site that
//     accepts only Equalizer's address), or destinations the platform
//     allows (EQ_DIRECT_DESTINATIONS, lab applications); never a name or a
//     private address, or a map could point the gateway at the platform's
//     own services;
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
			if !s.DirectDestinations[normDest(a)] && !publicDest(a) {
				return &reachError{"direct_not_allowed",
					"direct mode reaches a site at a public IP address and port, such as 203.0.113.10:443; use a connector for a site on a private network"}
			}
		}
	}
	return nil
}

// publicDest: a public IP address and port, written as such. A name could
// later resolve to the platform's own network; an address cannot.
func publicDest(d string) bool {
	ap, err := netip.ParseAddrPort(strings.TrimSpace(d))
	if err != nil || ap.Port() == 0 {
		return false
	}
	_, ok := exposure.PublicIP(ap.Addr().String())
	return ok
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

// newReach is the legacy.reach of a site without a map yet: through one of
// the tenant's connectors (mode "connector", the default) or directly at a
// public address (mode "direct"). checkReach then applies the usual rules.
func newReach(mode, connector, address string) (json.RawMessage, error) {
	addr := normDest(address)
	if addr == "" {
		return nil, &reachError{"invalid", "address (host:port) is required"}
	}
	switch mode {
	case "", "connector":
		if !connectorIDRe.MatchString(connector) {
			return nil, &reachError{"invalid", "connector_id is required"}
		}
		return json.Marshal(map[string]any{"mode": "connector", "connector_id": connector, "address": addr, "allow": []string{addr}})
	case "direct":
		return json.Marshal(map[string]any{"mode": "direct", "address": addr, "allow": []string{addr}})
	}
	return nil, &reachError{"invalid", "mode: connector or direct"}
}
