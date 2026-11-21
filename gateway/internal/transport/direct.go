// Package transport carries the rebuilt requests to the legacy application.
//
// Direct connects the gateway to a legacy application on the same network
// (direct mode); Relay sends the requests through tunnel-relay and the
// connector (connector mode). Both are http.RoundTripper.
package transport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/thevidocq/equalizer/gateway/internal/publicip"
)

// ErrDestinationNotAllowed reports a connection attempt outside the allowlist.
var ErrDestinationNotAllowed = errors.New("destination not allowed")

// Direct returns a RoundTripper that connects only to the host:port
// destinations listed in allow (legacy.reach.allow). No proxy, and no
// connections to other hosts even if a redirect or DNS asked for one.
// With address set, every connection goes there (it must be allowed): the
// URL's host still goes in Host and TLS SNI, for virtual hosts.
//
// A destination may be a public IP literal or a name (a site behind a CDN).
// A name is resolved at dial time and the connection goes only to a public
// address: a name that resolves, now or later, to a private or reserved
// address is refused, so DNS rebinding cannot reach the platform's own
// network.
func Direct(allow []string, address string, timeout time.Duration) (http.RoundTripper, error) {
	if len(allow) == 0 {
		return nil, errors.New("empty destination allowlist")
	}
	allowed := make(map[string]bool, len(allow))
	for _, a := range allow {
		host, port, err := net.SplitHostPort(a)
		if err != nil {
			return nil, fmt.Errorf("invalid destination %q: %w", a, err)
		}
		allowed[net.JoinHostPort(strings.ToLower(host), port)] = true
	}
	if address != "" {
		address = normalize(address)
		if !allowed[address] {
			return nil, fmt.Errorf("legacy.reach.address %q is not in allow", address)
		}
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	return &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if address != "" {
				addr = address
			}
			host, port, err := net.SplitHostPort(addr)
			if err != nil || !allowed[net.JoinHostPort(strings.ToLower(host), port)] {
				return nil, fmt.Errorf("%w: %s", ErrDestinationNotAllowed, addr)
			}
			return safeDial(ctx, dialer, network, host, port)
		},
		MaxIdleConnsPerHost:   32,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: timeout,
		TLSHandshakeTimeout:   5 * time.Second,
		ForceAttemptHTTP2:     false, // HTTP/1.1 only towards the legacy application
	}, nil
}

// resolveHost is the name resolver; tests replace it.
var resolveHost = func(ctx context.Context, host string) ([]netip.Addr, error) {
	return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
}

// safeDial connects to host:port. An IP literal is dialed as it is: it is
// already in the explicit allowlist, the operator's own decision, so a lab
// application on a private address stays reachable. A name introduces DNS
// indirection the operator cannot pin, so it is resolved and the connection
// goes only to a public address; a name that resolves to a private or
// reserved address is refused. This is the last barrier against rebinding
// to the platform's own network, even if config-time validation passed.
func safeDial(ctx context.Context, dialer *net.Dialer, network, host, port string) (net.Conn, error) {
	if _, err := netip.ParseAddr(host); err == nil {
		return dialer.DialContext(ctx, network, net.JoinHostPort(host, port))
	}
	ips, err := resolveHost(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve %q: %w", host, err)
	}
	var lastErr error
	for _, ip := range ips {
		ip = ip.Unmap()
		if !publicip.Public(ip) {
			lastErr = fmt.Errorf("%w: %s resolves to non-public %s", ErrDestinationNotAllowed, host, ip)
			continue
		}
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("%w: %s has no public address", ErrDestinationNotAllowed, host)
	}
	return nil, lastErr
}

// normalize lower-cases the host of a host:port ("" when malformed).
func normalize(d string) string {
	host, port, err := net.SplitHostPort(d)
	if err != nil {
		return ""
	}
	return net.JoinHostPort(strings.ToLower(host), port)
}
