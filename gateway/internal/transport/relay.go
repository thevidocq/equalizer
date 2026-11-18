package transport

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
)

// Errors reported by the relay or the connector (header X-Eq-Error).
var (
	ErrConnectorUnavailable = errors.New("connector unavailable")
	ErrLegacyUnreachable    = errors.New("legacy application unreachable through the connector")
	ErrRelay                = errors.New("relay error")
)

// RelayConfig says how to reach tunnel-relay on the control network.
type RelayConfig struct {
	URL   *url.URL // e.g. http://tunnel-relay:8082
	Token string
}

// ParseRelay validates the relay configuration. An empty URL means no relay.
func ParseRelay(raw, token string) (*RelayConfig, error) {
	if raw == "" {
		return nil, nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("invalid relay URL %q", raw)
	}
	if len(token) < 32 {
		return nil, errors.New("the relay token must be at least 32 characters")
	}
	return &RelayConfig{URL: u, Token: token}, nil
}

// Relay returns a RoundTripper that sends the rebuilt requests to the
// relay, which forwards them into the connector's tunnel
// (schema/tunnel/tunnel.md, section 3). Only destinations listed in allow
// (legacy.reach.allow) are sent; the connector checks its own allowlist too.
// tenant (the map's) lets the relay refuse another tenant's connector.
// With address set, the destination is always address and the URL's host
// travels in X-Eq-Host, for the connector to present (virtual hosts).
func Relay(cfg *RelayConfig, connectorID, tenant string, allow []string, address string, timeout time.Duration) (http.RoundTripper, error) {
	if cfg == nil {
		return nil, errors.New("no relay configured (EQ_RELAY_URL)")
	}
	if connectorID == "" {
		return nil, errors.New("legacy.reach.connector_id is required in connector mode")
	}
	if tenant == "" {
		return nil, errors.New("the map has no meta.tenant")
	}
	allowed, err := allowSet(allow)
	if err != nil {
		return nil, err
	}
	if address != "" {
		address = normalize(address)
		if !allowed[address] {
			return nil, fmt.Errorf("legacy.reach.address %q is not in allow", address)
		}
	}
	return &relayTransport{
		cfg: cfg, connector: connectorID, tenant: tenant, address: address, allowed: allowed,
		inner: &http.Transport{
			Proxy:                 nil,
			DialContext:           (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			MaxIdleConnsPerHost:   64,
			IdleConnTimeout:       90 * time.Second,
			ResponseHeaderTimeout: timeout + 5*time.Second, // the relay adds its own hop
			DisableCompression:    true,
		},
	}, nil
}

type relayTransport struct {
	cfg       *RelayConfig
	connector string
	tenant    string
	address   string
	allowed   map[string]bool
	inner     http.RoundTripper
}

func (t *relayTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	dest := destination(req.URL)
	if t.address != "" {
		dest = t.address
	}
	if !t.allowed[dest] {
		closeBody(req)
		return nil, fmt.Errorf("%w: %s", ErrDestinationNotAllowed, dest)
	}
	out := req.Clone(req.Context())
	target := *t.cfg.URL
	target.Path, target.RawPath = joinRaw(t.cfg.URL, req.URL)
	target.RawQuery = req.URL.RawQuery
	target.Fragment = ""
	out.URL = &target
	out.Host = t.cfg.URL.Host
	out.Header.Set("Authorization", "Bearer "+t.cfg.Token)
	out.Header.Set("X-Eq-Connector", t.connector)
	out.Header.Set("X-Eq-Tenant", t.tenant)
	if t.address != "" {
		out.Header.Set("X-Eq-Host", strings.ToLower(req.URL.Host))
	}
	out.Header.Set("X-Eq-Destination", dest)
	out.Header.Set("X-Eq-Scheme", req.URL.Scheme)
	if id := telemetry.RequestID(req.Context()); id != "" {
		out.Header.Set(telemetry.HeaderRequestID, id)
	}
	resp, err := t.inner.RoundTrip(out)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConnectorUnavailable, err)
	}
	if reason := resp.Header.Get("X-Eq-Error"); reason != "" {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		return nil, relayError(reason)
	}
	return resp, nil
}

// joinRaw keeps the already escaped legacy path as it is (a "/" encoded
// inside a value must stay encoded).
func joinRaw(base, u *url.URL) (string, string) {
	prefix := strings.TrimSuffix(base.Path, "/")
	rawPrefix := strings.TrimSuffix(base.EscapedPath(), "/")
	return prefix + u.Path, rawPrefix + u.EscapedPath()
}

func relayError(reason string) error {
	switch reason {
	case "connector_unavailable":
		return ErrConnectorUnavailable
	case "destination_not_allowed", "destination_not_declared":
		return fmt.Errorf("%w: %s", ErrDestinationNotAllowed, reason)
	case "legacy_unreachable", "legacy_timeout":
		return fmt.Errorf("%w: %s", ErrLegacyUnreachable, reason)
	}
	return fmt.Errorf("%w: %s", ErrRelay, sanitize(reason))
}

// destination returns host:port, with the scheme's default port.
func destination(u *url.URL) string {
	port := u.Port()
	if port == "" {
		port = "80"
		if u.Scheme == "https" {
			port = "443"
		}
	}
	return net.JoinHostPort(strings.ToLower(u.Hostname()), port)
}

func allowSet(allow []string) (map[string]bool, error) {
	if len(allow) == 0 {
		return nil, errors.New("empty destination allowlist")
	}
	out := make(map[string]bool, len(allow))
	for _, a := range allow {
		host, port, err := net.SplitHostPort(a)
		if err != nil {
			return nil, fmt.Errorf("invalid destination %q: %w", a, err)
		}
		out[net.JoinHostPort(strings.ToLower(host), port)] = true
	}
	return out, nil
}

func closeBody(req *http.Request) {
	if req.Body != nil {
		req.Body.Close()
	}
}

func sanitize(s string) string {
	if len(s) > 64 {
		s = s[:64]
	}
	return strings.Map(func(r rune) rune {
		if r < 0x21 || r > 0x7e {
			return -1
		}
		return r
	}, s)
}
