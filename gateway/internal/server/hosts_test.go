package server

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/clientip"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
)

// hostMaps serves one site on one host.
type hostMaps struct {
	host string
	s    *site.Site
}

func (h hostMaps) For(host string) *site.Site {
	if host == h.host {
		return h.s
	}
	return nil
}
func (h hostMaps) Ready() bool               { return true }
func (h hostMaps) All() []*site.Site         { return []*site.Site{h.s} }
func (h hostMaps) Serves(domain string) bool { return domain == h.host }

func TestUnknownHost(t *testing.T) {
	st := testSite(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "<p>home</p>") }))
	var events bytes.Buffer
	p := (&Public{Maps: hostMaps{"portal.example.org", st}, IPs: clientip.New(nil),
		Log: slog.New(slog.NewJSONHandler(io.Discard, nil)), Events: telemetry.NewEmitter(&events, "test")}).Handler()

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "portal.example.org"
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("mapped host: %d", rec.Code)
	}
	req.Host = "blog.example.org"
	rec = httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code != 404 || !strings.Contains(events.String(), "unknown_host") {
		t.Fatalf("unknown host: %d %s", rec.Code, events.String())
	}

	// Caddy never gets a certificate of its own for a protected site; it gets
	// the site's own, uploaded in the panel, only for a domain served here.
	certs := fixedCerts{"portal.example.org": []byte("PEM"), "evil.example.org": []byte("PEM")}
	admin := (&Admin{Maps: hostMaps{"portal.example.org", st}, Certs: certs}).Handler()
	for target, want := range map[string]int{
		"/tls/allowed?domain=portal.example.org":           404,
		"/tls/certificate?server_name=portal.example.org":  200,
		"/tls/certificate?server_name=PORTAL.example.org.": 200,
		"/tls/certificate?server_name=evil.example.org":    204, // a certificate, but no site here
		"/tls/certificate?server_name=other.example.org":   204,
	} {
		rec := httptest.NewRecorder()
		admin.ServeHTTP(rec, httptest.NewRequest("GET", target, nil))
		if rec.Code != want || (want == 200 && rec.Body.String() != "PEM") {
			t.Errorf("%s: %d", target, rec.Code)
		}
	}
}

type fixedCerts map[string][]byte

func (f fixedCerts) Bundle(d string) []byte {
	return f[strings.ToLower(strings.TrimSuffix(d, "."))]
}
