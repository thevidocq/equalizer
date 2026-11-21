package publicip

import (
	"net/netip"
	"testing"
)

func TestPublic(t *testing.T) {
	cases := map[string]bool{
		"8.8.8.8":              true,
		"1.1.1.1":              true,
		"2606:4700:4700::1111": true,
		"10.0.0.1":             false,
		"172.18.0.5":           false,
		"192.168.1.1":          false,
		"127.0.0.1":            false,
		"169.254.169.254":      false, // cloud metadata
		"100.64.0.1":           false, // CGNAT
		"203.0.113.10":         false, // documentation
		"::1":                  false,
		"fe80::1":              false,
		"224.0.0.1":            false, // multicast
		"0.0.0.0":              false,
	}
	for s, want := range cases {
		a, err := netip.ParseAddr(s)
		if err != nil {
			t.Fatalf("parse %q: %v", s, err)
		}
		if got := Public(a); got != want {
			t.Errorf("Public(%s) = %v, want %v", s, got, want)
		}
	}
}
