package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/clientip"
	"github.com/thevidocq/equalizer/gateway/internal/formtoken"
	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/session"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
)

// The block editor asks the REST API what the user may do with OPTIONS:
// the answer must be WordPress's, with its addresses made public.
func TestOptionsReachTheApplication(t *testing.T) {
	var legacyURL string
	hits := 0
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Method != http.MethodOptions {
			t.Errorf("method %s", r.Method)
		}
		w.Header().Set("Allow", "GET, POST")
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		_, _ = io.WriteString(w, `{"methods":["GET","POST"],"href":"`+strings.ReplaceAll(legacyURL, "/", `\/`)+`\/wp-json\/"}`)
	}))
	defer legacy.Close()
	legacyURL = legacy.URL
	build := func(methods ...string) *Public { return areaGateway(t, legacy.URL, methods...) }
	do := func(p *Public, hdr ...string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodOptions, "/wp-json/wp/v2/posts", nil)
		req.Host = "blog.example"
		for i := 0; i+1 < len(hdr); i += 2 {
			req.Header.Set(hdr[i], hdr[i+1])
		}
		rec := httptest.NewRecorder()
		p.Handler().ServeHTTP(rec, req)
		return rec
	}

	p := build("GET", "POST", "OPTIONS")
	rec := do(p)
	if rec.Code != 200 || rec.Header().Get("Allow") != "GET, POST" ||
		rec.Body.String() != `{"methods":["GET","POST"],"href":"https:\/\/blog.example\/wp-json\/"}` {
		t.Fatalf("OPTIONS through the area: %d %q %s", rec.Code, rec.Header().Get("Allow"), rec.Body.String())
	}
	// A CORS preflight is never sent on.
	if rec := do(p, "Origin", "https://evil.example", "Access-Control-Request-Method", "DELETE"); rec.Code != 204 || hits != 1 {
		t.Fatalf("preflight: %d, legacy hits %d", rec.Code, hits)
	}
	// An area without OPTIONS: answered by the gateway, as before.
	if rec := do(build("GET", "POST")); rec.Code != 204 || hits != 1 {
		t.Fatalf("area without OPTIONS: %d, legacy hits %d", rec.Code, hits)
	}
}

func areaGateway(t *testing.T, legacyURL string, methods ...string) *Public {
	t.Helper()
	u, _ := url.Parse(legacyURL)
	m := &mapmodel.Model{
		Meta:   mapmodel.Meta{Tenant: "t", App: "blog", Version: 1, Status: "published", PublicDomain: "blog.example"},
		Legacy: mapmodel.Legacy{BaseURL: legacyURL, Reach: mapmodel.Reach{Mode: "direct", Allow: []string{u.Host}}},
		Roles:  mapmodel.Roles{Anonymous: "guest", List: map[string]any{"guest": nil}},
		Areas:  []mapmodel.Area{{ID: "site", Prefix: "/", Methods: methods}},
	}
	st, err := site.Build(m)
	if err != nil {
		t.Fatal(err)
	}
	sealer, _ := formtoken.New(formtoken.RandomKey())
	mgr, _ := session.NewManager(session.NewMemory(), formtoken.RandomKey())
	return &Public{Maps: fixedMaps{st}, IPs: clientip.New(nil), Forms: sealer, Sessions: mgr,
		Log: slog.New(slog.NewJSONHandler(io.Discard, nil)), Events: telemetry.NewEmitter(io.Discard, "test")}
}

// A form showing the public address (WordPress's site address) must save
// the legacy application's own value.
func TestPublicOriginBackToLegacy(t *testing.T) {
	var got string
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = r.URL.RawQuery + " " + string(b)
		w.Header().Set("Content-Type", "text/plain")
	}))
	defer legacy.Close()
	p := areaGateway(t, legacy.URL, "GET", "POST")
	req := httptest.NewRequest("POST", "/wp-admin/options.php?redirect_to=https%3A%2F%2Fblog.example%2Fwp-admin%2F",
		strings.NewReader("siteurl=https%3A%2F%2Fblog.example&other=https%3A%2F%2Fblog.example.evil.test"))
	req.Host = "blog.example"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	p.Handler().ServeHTTP(httptest.NewRecorder(), req)
	enc := strings.NewReplacer(":", "%3A", "/", "%2F").Replace(legacy.URL)
	want := "redirect_to=" + enc + "%2Fwp-admin%2F siteurl=" + enc + "&other=https%3A%2F%2Fblog.example.evil.test"
	if got != want {
		t.Fatalf("legacy got\n %s\nwant\n %s", got, want)
	}
}
