// Package publicip classifies an IP address as safe for the gateway to dial
// in direct mode: a public unicast address, never one on the platform's own
// or any reserved network. It is the dial-time barrier against a name that
// resolves (or is rebound) to an internal address.
package publicip

import "net/netip"

// reserved lists ranges that are routable-looking but must never be dialed.
var reserved = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"),   // carrier-grade NAT
	netip.MustParsePrefix("192.0.0.0/24"),    // IETF protocol assignments
	netip.MustParsePrefix("192.0.2.0/24"),    // documentation
	netip.MustParsePrefix("198.18.0.0/15"),   // benchmarking
	netip.MustParsePrefix("198.51.100.0/24"), // documentation
	netip.MustParsePrefix("203.0.113.0/24"),  // documentation
	netip.MustParsePrefix("240.0.0.0/4"),     // reserved
	netip.MustParsePrefix("2001:db8::/32"),   // documentation
	netip.MustParsePrefix("64:ff9b::/96"),    // NAT64
}

// Public reports whether a is a public unicast address the gateway may dial.
// It rejects private, loopback, link-local, multicast, unspecified and the
// reserved ranges above. The caller passes an unmapped address.
func Public(a netip.Addr) bool {
	if !a.IsValid() || a.Zone() != "" {
		return false
	}
	a = a.Unmap()
	if !a.IsGlobalUnicast() || a.IsPrivate() || a.IsLoopback() || a.IsLinkLocalUnicast() || a.IsMulticast() {
		return false
	}
	for _, p := range reserved {
		if p.Contains(a) {
			return false
		}
	}
	return true
}
