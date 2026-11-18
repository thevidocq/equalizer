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
	"strings"
	"time"
)

// ErrDestinationNotAllowed reports a connection attempt outside the allowlist.
var ErrDestinationNotAllowed = errors.New("destination not allowed")

// Direct returns a RoundTripper that connects only to the host:port
// destinations listed in allow (legacy.reach.allow). No proxy, and no
// connections to other hosts even if a redirect or DNS asked for one.
// With address set, every connection goes there (it must be allowed): the
// URL's host still goes in Host and TLS SNI, for virtual hosts.
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
			return dialer.DialContext(ctx, network, addr)
		},
		MaxIdleConnsPerHost:   32,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: timeout,
		TLSHandshakeTimeout:   5 * time.Second,
		ForceAttemptHTTP2:     false, // HTTP/1.1 only towards the legacy application
	}, nil
}

// normalize lower-cases the host of a host:port ("" when malformed).
func normalize(d string) string {
	host, port, err := net.SplitHostPort(d)
	if err != nil {
		return ""
	}
	return net.JoinHostPort(strings.ToLower(host), port)
}
