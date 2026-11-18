package mapstore

import (
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/thevidocq/equalizer/gateway/internal/site"
)

// Sites is an immutable set of compiled maps, by public domain. A single map
// without meta.public_domain (signed by hand, lab) is served for every host.
type Sites struct {
	byHost map[string]*site.Site
	single *site.Site
}

// For returns the site served on host (a Host header, port allowed), or nil.
func (x *Sites) For(host string) *site.Site {
	if x.single != nil {
		return x.single
	}
	return x.byHost[normalizeHost(host)]
}

// Len is the number of sites.
func (x *Sites) Len() int {
	if x.single != nil {
		return 1
	}
	return len(x.byHost)
}

// All returns the sites ordered by app.
func (x *Sites) All() []*site.Site {
	if x.single != nil {
		return []*site.Site{x.single}
	}
	out := make([]*site.Site, 0, len(x.byHost))
	for _, s := range x.byHost {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Model.Meta.App < out[j].Model.Meta.App })
	return out
}

func (x *Sites) byApp(app string) *site.Site {
	if app == "" {
		return nil
	}
	for _, s := range x.All() {
		if s.Model.Meta.App == app {
			return s
		}
	}
	return nil
}

func (x *Sites) add(s *site.Site) error {
	d := normalizeHost(s.Model.Meta.PublicDomain)
	if d == "" {
		if x.single != nil {
			return fmt.Errorf("map of %s has no meta.public_domain", s.Model.Meta.App)
		}
		x.single = s
		return nil
	}
	if other := x.byHost[d]; other != nil {
		return fmt.Errorf("maps of %s and %s share the domain %s", other.Model.Meta.App, s.Model.Meta.App, d)
	}
	x.byHost[d] = s
	return nil
}

// check refuses a map without domain mixed with others: it would capture
// every host.
func (x *Sites) check() error {
	if x.single != nil && len(x.byHost) > 0 {
		return fmt.Errorf("map of %s has no meta.public_domain and other maps are loaded", x.single.Model.Meta.App)
	}
	return nil
}

// normalizeHost lower-cases and drops the port and a final dot.
func normalizeHost(h string) string {
	h = strings.ToLower(strings.TrimSpace(h))
	if host, _, err := net.SplitHostPort(h); err == nil {
		h = host
	}
	return strings.TrimSuffix(h, ".")
}
