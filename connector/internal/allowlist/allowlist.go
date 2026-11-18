// Package allowlist holds the destinations the connector may reach. It is
// configured locally (EQ_ALLOW) by whoever runs the connector: neither the
// relay nor the gateway can widen it.
package allowlist

import (
	"fmt"
	"net"
	"strings"
)

// List is an immutable set of exact host:port destinations.
type List struct {
	items []string
	set   map[string]bool
}

// Parse reads "host:port,host:port". Hosts are compared in lower case.
func Parse(raw string) (*List, error) {
	l := &List{set: map[string]bool{}}
	for _, d := range strings.Split(raw, ",") {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == "" {
			continue
		}
		host, port, err := net.SplitHostPort(d)
		if err != nil || host == "" || port == "" {
			return nil, fmt.Errorf("invalid destination %q: use host:port", d)
		}
		if !l.set[d] {
			l.set[d] = true
			l.items = append(l.items, d)
		}
	}
	if len(l.items) == 0 {
		return nil, fmt.Errorf("the allowlist is empty")
	}
	return l, nil
}

// Contains reports whether destination is allowed (exact host:port).
func (l *List) Contains(destination string) bool {
	return l.set[strings.ToLower(destination)]
}

// Items returns the destinations in their declared order.
func (l *List) Items() []string { return append([]string(nil), l.items...) }
