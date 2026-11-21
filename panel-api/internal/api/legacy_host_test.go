package api

import "testing"

func TestLegacyHost(t *testing.T) {
	cases := []struct{ mode, addr, override, want string }{
		{"direct", "www.metreta.com:443", "", "www.metreta.com"},                   // name: sent as is (CDN)
		{"direct", "WWW.Metreta.com.:443", "", "www.metreta.com"},                  // normalised
		{"direct", "203.0.113.10:443", "", "site.example.com"},                     // IP: public domain
		{"direct", "[2606:4700::1]:443", "", "site.example.com"},                   // IPv6: public domain
		{"connector", "127.0.0.1:8080", "", "site.example.com"},                    // connector: public domain
		{"connector", "intranet.local:8080", "", "site.example.com"},               // connector name: public domain
		{"direct", "origin.metreta.com:443", "www.metreta.com", "www.metreta.com"}, // override wins
	}
	for _, c := range cases {
		got, err := legacyHost("site.example.com", c.mode, c.addr, c.override)
		if err != nil || got != c.want {
			t.Errorf("legacyHost(%q,%q,%q) = %q, %v; want %q", c.mode, c.addr, c.override, got, err, c.want)
		}
	}
	if _, err := legacyHost("site.example.com", "direct", "8.8.8.8:443", "not a host!"); err == nil {
		t.Error("invalid override accepted")
	}
}
