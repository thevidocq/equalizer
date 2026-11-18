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

	admin := (&Admin{Maps: hostMaps{"portal.example.org", st}}).Handler()
	for domain, want := range map[string]int{"portal.example.org": 200, "evil.example.org": 404} {
		rec := httptest.NewRecorder()
		admin.ServeHTTP(rec, httptest.NewRequest("GET", "/tls/allowed?domain="+domain, nil))
		if rec.Code != want {
			t.Errorf("tls/allowed %s: %d", domain, rec.Code)
		}
	}
}
