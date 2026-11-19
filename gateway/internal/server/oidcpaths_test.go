package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/clientip"
	"github.com/thevidocq/equalizer/gateway/internal/formtoken"
	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/session"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
)

func TestLoginPath(t *testing.T) {
	for in, want := range map[string]string{
		"/wp-admin/":              "/wp-admin/",
		"/WP-Admin/index.php":     "/wp-admin/index.php",
		"//wp-admin/":             "/wp-admin/",
		"/x/../wp-admin/":         "/wp-admin/",
		"/wp-admin;jsessionid=1/": "/wp-admin/",
		"/wp-login.php.":          "/wp-login.php",
		"/wp-login.php%20":        "/wp-login.php%20",
		"\\wp-admin\\":            "/wp-admin/",
		"/":                       "/",
	} {
		if got := loginPath(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}

func TestOIDCPaths(t *testing.T) {
	idp := newIdP(t)
	var auth string
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, "<p>page</p>")
	}))
	defer legacy.Close()
	u, _ := url.Parse(legacy.URL)
	m := &mapmodel.Model{
		Meta:   mapmodel.Meta{Tenant: "t", App: "portal", Version: 1, Status: "published", PublicDomain: "portal.acme.example"},
		Legacy: mapmodel.Legacy{BaseURL: legacy.URL, Reach: mapmodel.Reach{Mode: "direct", Allow: []string{u.Host}}},
		Roles: mapmodel.Roles{Anonymous: "guest", List: map[string]any{"guest": nil, "staff": nil},
			Auth: &mapmodel.Auth{Provider: "oidc", Issuer: idp.srv.URL, ClientID: "eq-portal",
				Roles: []mapmodel.AuthRole{{Group: "*", Role: "staff"}},
				Paths: []string{"/wp-admin/", "/wp-login.php", "/xmlrpc.php"}}},
		Areas: []mapmodel.Area{{ID: "site", Prefix: "/", Methods: []string{"GET", "POST"}}},
	}
	st, err := site.Build(m)
	if err != nil {
		t.Fatal(err)
	}
	st.OIDC.Client = idp.srv.Client()
	sealer, _ := formtoken.New(formtoken.RandomKey())
	mgr, _ := session.NewManager(session.NewMemory(), formtoken.RandomKey())
	p := &Public{Maps: fixedMaps{st}, IPs: clientip.New(nil), Forms: sealer, Sessions: mgr, OIDCSecrets: oneSecret("s"),
		Log: slog.New(slog.NewJSONHandler(io.Discard, nil)), Events: telemetry.NewEmitter(io.Discard, "test")}
	b := &browser{t: t, h: p.Handler(), cookies: map[string]string{},
		hdr: http.Header{"Authorization": {"Basic YWRtaW46cHc="}}} // an application password

	// The public site needs no login, and an anonymous HTTP login never
	// reaches the legacy site.
	if rec := b.do("GET", "/blog/hello/", nil); rec.Code != 200 || auth != "" {
		t.Fatalf("public page: %d, Authorization %q", rec.Code, auth)
	}
	for _, path := range []string{"/wp-admin/", "/wp-admin", "/WP-ADMIN/users.php", "//wp-admin/", "/x/../wp-admin/",
		"/wp-admin;x/", "/wp-login.php", "/wp-login.php.", "/wp-admin%2Fusers.php"} {
		if rec := b.do("GET", path, nil); rec.Code != http.StatusFound {
			t.Errorf("%s: %d, want the login", path, rec.Code)
		}
	}
	if rec := b.do("POST", "/xmlrpc.php", url.Values{"x": {"1"}}); rec.Code != http.StatusUnauthorized {
		t.Errorf("POST xmlrpc.php: %d", rec.Code)
	}
	if rec := b.do("GET", "/wp-adminer/", nil); rec.Code != 200 {
		t.Errorf("a path that only starts alike: %d", rec.Code)
	}

	// After the provider: admin pages open, and the HTTP login passes.
	rec := b.do("GET", "/wp-admin/", nil)
	if rec = b.do("GET", idp.authorize(t, rec.Header().Get("Location")), nil); rec.Code != http.StatusFound {
		t.Fatalf("callback: %d", rec.Code)
	}
	if rec := b.do("GET", "/wp-admin/", nil); rec.Code != 200 || auth != "Basic YWRtaW46cHc=" {
		t.Errorf("after login: %d, Authorization %q", rec.Code, auth)
	}
}

type aliasMaps struct{ s *site.Site }

func (a aliasMaps) For(string) *site.Site { return nil }
func (a aliasMaps) Ready() bool           { return true }
func (a aliasMaps) AliasOf(h string) *site.Site {
	if h == "example.org" {
		return a.s
	}
	return nil
}

func TestAliasRedirect(t *testing.T) {
	st := &site.Site{Model: &mapmodel.Model{Meta: mapmodel.Meta{App: "portal", PublicDomain: "www.example.org"}}}
	p := &Public{Maps: aliasMaps{st}, IPs: clientip.New(nil), Log: slog.New(slog.NewJSONHandler(io.Discard, nil)),
		Events: telemetry.NewEmitter(io.Discard, "test")}
	for _, c := range []struct {
		method, target, host, want string
		code                       int
	}{
		{"GET", "/news/?id=3", "example.org", "https://www.example.org/news/?id=3", http.StatusMovedPermanently},
		{"POST", "/contact", "example.org", "https://www.example.org/contact", http.StatusPermanentRedirect},
		{"GET", "//evil.example/x", "example.org", "https://www.example.org/", http.StatusMovedPermanently},
		{"GET", "/", "other.example.org", "", http.StatusNotFound},
	} {
		req := httptest.NewRequest(c.method, c.target, nil)
		req.Host = c.host
		rec := httptest.NewRecorder()
		p.Handler().ServeHTTP(rec, req)
		if rec.Code != c.code || rec.Header().Get("Location") != c.want {
			t.Errorf("%s %s%s: %d %q", c.method, c.host, c.target, rec.Code, rec.Header().Get("Location"))
		}
	}
}
