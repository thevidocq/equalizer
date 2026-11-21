package api

import (
	"net"
	"net/netip"
	"strings"
)

// legacyHost is the name the legacy site is asked for: the Host header and
// TLS SNI that Equalizer sends, and the name its certificate is checked
// against (the host of legacy.base_url).
//
//   - an explicit override (advanced; e.g. a dedicated origin hostname whose
//     server answers only to the public name) wins;
//   - in direct mode with a name as address, that name: a site behind a CDN
//     (Cloudflare and the like) answers only to its own name, so a parallel
//     public domain (metreta.example.com in front of www.metreta.com) works
//     with no other setting;
//   - otherwise the public domain, as before (connector, or a direct IP).
func legacyHost(publicDomain, mode, address, override string) (string, error) {
	if o := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(override)), "."); o != "" {
		if !domainRe.MatchString(o) || len(o) > 253 {
			return "", &reachError{"invalid", "legacy_host: a domain name, such as www.example.com"}
		}
		return o, nil
	}
	if mode == "direct" {
		if host, _, err := net.SplitHostPort(strings.TrimSpace(address)); err == nil && host != "" {
			if _, err := netip.ParseAddr(strings.Trim(host, "[]")); err != nil {
				return strings.TrimSuffix(strings.ToLower(host), "."), nil
			}
		}
	}
	return publicDomain, nil
}
