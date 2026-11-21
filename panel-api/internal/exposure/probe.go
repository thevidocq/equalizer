// Package exposure checks whether a protected site still answers directly on
// its old public IP, bypassing Equalizer: the probe connects to that IP on
// ports 443 and 80 presenting the site's name (TLS SNI and Host), as an
// attacker who knows the old address would.
package exposure

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"regexp"
	"strings"
	"time"
)

// Statuses and reasons (schema/api/panel-api.yaml, Exposure).
const (
	Closed  = "closed"
	Exposed = "exposed"
	Error   = "error"

	ReasonRefused      = "refused"
	ReasonTimeout      = "timeout"
	ReasonForbidden    = "forbidden"
	ReasonDefaultSite  = "default_site"
	ReasonRedirectOnly = "redirect_only"
	ReasonHTTPS        = "answers_https"
	ReasonHTTP         = "answers_http"
	ReasonProbeFailed  = "probe_failed"
)

// Result is the outcome of one check.
type Result struct{ Status, Reason string }

// maxBody bounds what is read of each answer.
const maxBody = 256 << 10

// Prober runs checks. Dial connects to the given address; tests replace it.
type Prober struct {
	Timeout time.Duration
	Dial    func(ctx context.Context, network, addr string) (net.Conn, error)
}

// PublicIP parses s and accepts only a public unicast address: the probe
// must never reach the platform's own networks.
func PublicIP(s string) (netip.Addr, bool) {
	s = strings.TrimSpace(s)
	if ap, err := netip.ParseAddrPort(s); err == nil { // 203.0.113.10:80: the port is not needed
		s = ap.Addr().String()
	}
	a, err := netip.ParseAddr(s)
	if err != nil || a.Zone() != "" {
		return netip.Addr{}, false
	}
	a = a.Unmap()
	if !a.IsGlobalUnicast() || a.IsPrivate() || a.IsLoopback() || a.IsLinkLocalUnicast() {
		return netip.Addr{}, false
	}
	for _, p := range reserved {
		if p.Contains(a) {
			return netip.Addr{}, false
		}
	}
	return a, true
}

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

// Check probes ip for domain. It never follows redirects and reads at most
// maxBody of each answer; nothing of the answers is kept.
func (p *Prober) Check(ctx context.Context, ip, domain string) Result {
	addr, ok := PublicIP(ip)
	if !ok {
		return Result{Error, ReasonProbeFailed}
	}
	https := p.port(ctx, addr, "https", domain)
	if https.Status == Exposed {
		return https
	}
	plain := p.port(ctx, addr, "http", domain)
	if plain.Status == Exposed {
		return plain
	}
	if https.Status == Error && plain.Status == Error {
		return Result{Error, ReasonProbeFailed}
	}
	return https
}

// port checks one scheme. The site's answer is compared with the answer for
// a name the server cannot know: the same answer means the server only
// shows its default site.
func (p *Prober) port(ctx context.Context, addr netip.Addr, scheme, domain string) Result {
	site, err := p.fetch(ctx, addr, scheme, domain)
	if err != nil {
		return Result{Closed, failure(err)}
	}
	switch site.status {
	case http.StatusBadRequest, http.StatusForbidden, http.StatusMisdirectedRequest, 444:
		return Result{Closed, ReasonForbidden}
	}
	if site.status >= 300 && site.status < 400 && redirectsToSelf(site.location, domain) {
		return Result{Closed, ReasonRedirectOnly}
	}
	other, err := p.fetch(ctx, addr, scheme, randomName())
	if err == nil && site.same(other) {
		return Result{Closed, ReasonDefaultSite}
	}
	if scheme == "https" {
		return Result{Exposed, ReasonHTTPS}
	}
	return Result{Exposed, ReasonHTTP}
}

type answer struct {
	status   int
	location string
	length   int
	sum      string
	title    string
}

func (a answer) same(b answer) bool {
	if a.status != b.status {
		return false
	}
	if a.sum == b.sum {
		return true
	}
	// Dynamic pages differ a little between two loads.
	if a.title != b.title {
		return false
	}
	lo, hi := min(a.length, b.length), max(a.length, b.length)
	return hi == 0 || float64(lo)/float64(hi) > 0.9
}

func (p *Prober) fetch(ctx context.Context, addr netip.Addr, scheme, host string) (answer, error) {
	port := "443"
	if scheme == "http" {
		port = "80"
	}
	target := net.JoinHostPort(addr.String(), port)
	dial := p.Dial
	if dial == nil {
		dial = (&net.Dialer{}).DialContext
	}
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	tr := &http.Transport{
		Proxy: nil,
		// Always the old IP, whatever the name resolves to.
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dial(ctx, network, target)
		},
		// The point is reachability, not who signed the certificate.
		TLSClientConfig:     &tls.Config{ServerName: host, InsecureSkipVerify: true}, //nolint:gosec
		TLSHandshakeTimeout: timeout,
		DisableKeepAlives:   true,
		DisableCompression:  true,
	}
	defer tr.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, scheme+"://"+host+"/", nil)
	if err != nil {
		return answer{}, err
	}
	req.Header.Set("User-Agent", "equalizer-exposure-check")
	resp, err := tr.RoundTrip(req)
	if err != nil {
		return answer{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	sum := sha256.Sum256(body)
	return answer{
		status: resp.StatusCode, location: resp.Header.Get("Location"),
		length: len(body), sum: hex.EncodeToString(sum[:]), title: title(body),
	}, nil
}

var titleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

func title(body []byte) string {
	m := titleRe.FindSubmatch(body)
	if m == nil {
		return ""
	}
	return string(bytes.TrimSpace(m[1]))
}

// redirectsToSelf: an HTTP port that only sends visitors to the site's
// HTTPS name (now served by Equalizer) gives nothing away.
func redirectsToSelf(location, domain string) bool {
	l := strings.ToLower(location)
	return strings.HasPrefix(l, "https://"+domain+"/") || l == "https://"+domain
}

func failure(err error) string {
	var ne net.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &ne) && ne.Timeout():
		return ReasonTimeout
	default:
		// Refused, reset, TLS refused: nothing answers for this name.
		return ReasonRefused
	}
}

func randomName() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "eq-probe-" + hex.EncodeToString(b) + ".invalid"
}
