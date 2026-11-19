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
		{"a client's CF-Connecting-IP without Cloudflare", "172.20.0.2:4000", "198.51.100.7", "192.0.2.44", "198.51.100.7"},
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

func TestCloudflare(t *testing.T) {
	r := New([]netip.Prefix{netip.MustParsePrefix("172.20.0.0/16")})
	r.Cloudflare = true
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.20.0.2:4000"
	req.Header.Set("CF-Connecting-IP", "192.0.2.44")
	if got := r.IP(req); got != "192.0.2.44" {
		t.Errorf("behind Cloudflare: %q", got)
	}
}
