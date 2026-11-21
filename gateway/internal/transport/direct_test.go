package transport

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestDirectAddress(t *testing.T) {
	var host string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { host = r.Host }))
	defer srv.Close()
	addr := strings.TrimPrefix(srv.URL, "http://")
	rt, err := Direct([]string{addr}, addr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", "http://permits.example.org/", nil)
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if host != "permits.example.org" {
		t.Fatalf("Host %q: the site's name must reach the web server", host)
	}
	if _, err := Direct([]string{addr}, "10.0.0.1:80", time.Second); err == nil {
		t.Fatal("address outside allow accepted")
	}
	plain, _ := Direct([]string{addr}, "", time.Second)
	if _, err := plain.RoundTrip(req); !errors.Is(err, ErrDestinationNotAllowed) {
		t.Fatalf("without address the URL's host must be allowed: %v", err)
	}
}

// TestDirectNamePrivateRefused: a name destination, even in the allowlist,
// is dialed only at a public address. A name that resolves to a private or
// reserved address is refused, so DNS rebinding cannot reach the platform's
// own network.
func TestDirectNamePrivateRefused(t *testing.T) {
	orig := resolveHost
	defer func() { resolveHost = orig }()
	req, _ := http.NewRequest("GET", "https://origin.example.com/", nil)

	for _, ip := range []string{"10.0.0.5", "169.254.169.254", "127.0.0.1", "203.0.113.9"} {
		resolveHost = func(context.Context, string) ([]netip.Addr, error) {
			return []netip.Addr{netip.MustParseAddr(ip)}, nil
		}
		rt, err := Direct([]string{"origin.example.com:443"}, "origin.example.com:443", time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := rt.RoundTrip(req); !errors.Is(err, ErrDestinationNotAllowed) {
			t.Fatalf("a name resolving to %s must be refused, got: %v", ip, err)
		}
	}

	// No resolution at all: refused, not dialed.
	resolveHost = func(context.Context, string) ([]netip.Addr, error) {
		return nil, &net.DNSError{Err: "no such host", Name: "origin.example.com", IsNotFound: true}
	}
	rt, _ := Direct([]string{"origin.example.com:443"}, "origin.example.com:443", time.Second)
	if _, err := rt.RoundTrip(req); err == nil {
		t.Fatal("a name that does not resolve must fail")
	}
}
