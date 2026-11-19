// Package clientip determines the client's real IP address.
//
// X-Forwarded-For and CF-Connecting-IP are honoured only when the connection
// comes from a trusted network (the edge proxy, or Cloudflare if configured).
package clientip

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// Resolver determines the client IP while respecting the trusted networks.
type Resolver struct {
	trusted []netip.Prefix
	// Cloudflare: honour CF-Connecting-IP. Only when the edge really is
	// Cloudflare: otherwise any client could choose its own address.
	Cloudflare bool
}

// New creates a Resolver with the given trusted networks.
func New(trusted []netip.Prefix) *Resolver {
	return &Resolver{trusted: trusted}
}

// IP returns the client IP for the request.
func (r *Resolver) IP(req *http.Request) string {
	peer := peerAddr(req.RemoteAddr)
	if !peer.IsValid() || !r.isTrusted(peer) {
		return peer.String()
	}
	if v := strings.TrimSpace(req.Header.Get("CF-Connecting-IP")); r.Cloudflare && v != "" {
		if a, err := netip.ParseAddr(v); err == nil {
			return a.Unmap().WithZone("").String()
		}
	}
	// The last untrusted address in X-Forwarded-For is the client.
	// Every X-Forwarded-For line counts: a proxy may add its own line instead
	// of appending to the client's, and the client's must never win.
	parts := strings.Split(strings.Join(req.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(parts) - 1; i >= 0; i-- {
		a, err := netip.ParseAddr(strings.TrimSpace(parts[i]))
		if err != nil {
			break
		}
		a = a.Unmap().WithZone("")
		if !r.isTrusted(a) {
			return a.String()
		}
	}
	return peer.String()
}

func (r *Resolver) isTrusted(a netip.Addr) bool {
	for _, p := range r.trusted {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

func peerAddr(remote string) netip.Addr {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	a, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return a.Unmap()
}

// LimitKey is the address rate limits count by: the IPv4 address, or the
// /64 network of an IPv6 address (one subscriber gets a whole /64 and could
// otherwise change address at every request).
func LimitKey(ip string) string {
	a, err := netip.ParseAddr(ip)
	if err != nil || a.Is4() {
		return ip
	}
	p, err := a.Prefix(64)
	if err != nil {
		return ip
	}
	return p.String()
}
