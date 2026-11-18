// Package clientip ricava l'IP reale del client.
//
// X-Forwarded-For e CF-Connecting-IP sono accettati solo se la connessione
// arriva da una rete fidata (il proxy edge, o Cloudflare se configurato).
package clientip

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// Resolver ricava l'IP del client rispettando le reti fidate.
type Resolver struct {
	trusted []netip.Prefix
}

// New crea un Resolver con le reti fidate indicate.
func New(trusted []netip.Prefix) *Resolver {
	return &Resolver{trusted: trusted}
}

// IP restituisce l'IP del client per la richiesta.
func (r *Resolver) IP(req *http.Request) string {
	peer := peerAddr(req.RemoteAddr)
	if !peer.IsValid() || !r.isTrusted(peer) {
		return peer.String()
	}
	if v := strings.TrimSpace(req.Header.Get("CF-Connecting-IP")); v != "" {
		if a, err := netip.ParseAddr(v); err == nil {
			return a.String()
		}
	}
	// L'ultimo indirizzo di X-Forwarded-For non fidato è il client.
	parts := strings.Split(req.Header.Get("X-Forwarded-For"), ",")
	for i := len(parts) - 1; i >= 0; i-- {
		a, err := netip.ParseAddr(strings.TrimSpace(parts[i]))
		if err != nil {
			break
		}
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
