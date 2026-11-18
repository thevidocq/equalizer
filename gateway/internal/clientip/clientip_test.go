package clientip

import (
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestIP(t *testing.T) {
	r := New([]netip.Prefix{netip.MustParsePrefix("172.20.0.0/16")})
	cases := []struct {
		name, remote, xff, cf, want string
	}{
		{"direct, headers ignored", "203.0.113.5:4000", "1.2.3.4", "5.6.7.8", "203.0.113.5"},
		{"from trusted proxy", "172.20.0.2:4000", "198.51.100.7", "", "198.51.100.7"},
		{"chain of trusted proxies", "172.20.0.2:4000", "198.51.100.7, 172.20.0.9", "", "198.51.100.7"},
		{"cloudflare via trusted proxy", "172.20.0.2:4000", "", "192.0.2.44", "192.0.2.44"},
		{"malformed xff", "172.20.0.2:4000", "garbage", "", "172.20.0.2"},
	}
	for _, c := range cases {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = c.remote
		if c.xff != "" {
			req.Header.Set("X-Forwarded-For", c.xff)
		}
		if c.cf != "" {
			req.Header.Set("CF-Connecting-IP", c.cf)
		}
		if got := r.IP(req); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}
