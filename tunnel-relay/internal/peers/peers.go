// Package peers finds the other relay replicas. A connector's tunnel lives
// on one replica; a request that reaches another one is handed to its
// peers. Peers come from DNS (in Compose, the service name resolves to every
// replica), so the replicas share no state.
package peers

import (
	"context"
	"net"
	"time"
)

// Finder lists the internal addresses (host:port) of the other replicas.
type Finder struct {
	Name     string // DNS name of the replicas, e.g. tunnel-relay
	Port     string // their internal port
	Resolver *net.Resolver
	self     map[string]bool
}

// New prepares a finder; the replica's own addresses are excluded.
func New(name, port string) *Finder {
	f := &Finder{Name: name, Port: port, Resolver: net.DefaultResolver, self: map[string]bool{}}
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if ipn, ok := a.(*net.IPNet); ok {
			f.self[ipn.IP.String()] = true
		}
	}
	return f
}

// Peers returns the other replicas now.
func (f *Finder) Peers(ctx context.Context) []string {
	if f == nil || f.Name == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	ips, err := f.Resolver.LookupHost(ctx, f.Name)
	if err != nil {
		return nil
	}
	var out []string
	for _, ip := range ips {
		if !f.self[ip] {
			out = append(out, net.JoinHostPort(ip, f.Port))
		}
	}
	return out
}
