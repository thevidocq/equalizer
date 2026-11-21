package api

import (
	"context"
	"net/netip"
	"testing"
)

func TestCheckDirectDest(t *testing.T) {
	orig := lookupDirect
	defer func() { lookupDirect = orig }()

	resolve := func(addrs ...string) func(context.Context, string) ([]netip.Addr, error) {
		return func(context.Context, string) ([]netip.Addr, error) {
			out := make([]netip.Addr, len(addrs))
			for i, a := range addrs {
				out[i] = netip.MustParseAddr(a)
			}
			return out, nil
		}
	}

	// IP literals: public accepted, private/reserved refused (no lookup).
	lookupDirect = func(context.Context, string) ([]netip.Addr, error) {
		t.Fatal("literal must not resolve")
		return nil, nil
	}
	if err := checkDirectDest(context.Background(), "8.8.8.8:443"); err != nil {
		t.Fatalf("public IP literal refused: %v", err)
	}
	if err := checkDirectDest(context.Background(), "10.0.0.1:443"); err == nil {
		t.Fatal("private IP literal accepted")
	}
	if err := checkDirectDest(context.Background(), "example.com:0"); err == nil {
		t.Fatal("port 0 accepted")
	}
	if err := checkDirectDest(context.Background(), "example.com"); err == nil {
		t.Fatal("missing port accepted")
	}

	// Name resolving to public addresses only: accepted.
	lookupDirect = resolve("8.8.8.8", "1.1.1.1")
	if err := checkDirectDest(context.Background(), "origin.example.com:443"); err != nil {
		t.Fatalf("public name refused: %v", err)
	}

	// Name resolving to any private/reserved address: refused.
	for _, bad := range []string{"10.0.0.5", "169.254.169.254", "127.0.0.1", "203.0.113.9"} {
		lookupDirect = resolve("8.8.8.8", bad)
		if err := checkDirectDest(context.Background(), "origin.example.com:443"); err == nil {
			t.Fatalf("name resolving to %s accepted", bad)
		}
	}

	// Name that does not resolve: refused.
	lookupDirect = func(context.Context, string) ([]netip.Addr, error) { return nil, nil }
	if err := checkDirectDest(context.Background(), "origin.example.com:443"); err == nil {
		t.Fatal("unresolved name accepted")
	}
}
