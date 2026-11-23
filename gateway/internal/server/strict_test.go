package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/clientip"
	"github.com/thevidocq/equalizer/gateway/internal/formtoken"
	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/session"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
	"github.com/thevidocq/equalizer/gateway/internal/unmapped"
)

// A request that steps aside from a precise entry into the area keeps the
// entry's checks (efficacy study, 0.2.14): an extra key does not carry an
// attack past a declared one, a path value no page has is refused, and a
// file that only has the shape of a page goes to its asset prefix.
func TestAreaKeepsThePreciseChecks(t *testing.T) {
	var mu sync.Mutex
	var hits []string
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits = append(hits, r.URL.RequestURI())
		mu.Unlock()
		if r.URL.Path == "/wp-includes/js/x.js" {
			w.Header().Set("Content-Type", "application/javascript")
			_, _ = io.WriteString(w, "var x = 1;")
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, "<html><body>page</body></html>")
	}))
	defer legacy.Close()
	u, _ := url.Parse(legacy.URL)
	slug := mapmodel.TypeDef{Base: "string", Pattern: `^[^/.<>{}"'\\\x00-\x1f\x7f]{1,200}$`, Deny: []string{"wp-admin"}}
	m := &mapmodel.Model{
		Meta:   mapmodel.Meta{Tenant: "t", App: "blog", Version: 1, Status: "published", PublicDomain: "blog.example"},
		Legacy: mapmodel.Legacy{BaseURL: legacy.URL, Reach: mapmodel.Reach{Mode: "direct", Allow: []string{u.Host}}},
		Roles:  mapmodel.Roles{Anonymous: "guest", List: map[string]any{"guest": nil}},
		Types:  map[string]mapmodel.TypeDef{"slug": slug},
		Routes: []mapmodel.Route{
			{ID: "home", Path: "/", Method: "GET", Roles: []string{"guest"}, Query: map[string]mapmodel.FieldSpec{"p": {Type: "int"}},
				LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/", Query: map[string]string{"p": "{{query.p}}"}}},
			{ID: "page", Path: "/{slug}/", Method: "GET", Roles: []string{"guest"}, Params: map[string]mapmodel.FieldSpec{"slug": {Type: "slug"}},
				LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/{{params.slug}}/"}},
			{ID: "dated", Path: "/{year}/{month}/{slug}/", Method: "GET", Roles: []string{"guest"},
				Params:        map[string]mapmodel.FieldSpec{"year": {Type: "int"}, "month": {Type: "int"}, "slug": {Type: "slug"}},
				LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/{{params.year}}/{{params.month}}/{{params.slug}}/"}},
		},
		Assets: []mapmodel.Asset{{Prefix: "/wp-includes/", Types: []string{"application/javascript"}}},
		Areas:  []mapmodel.Area{{ID: "site", Prefix: "/"}},
	}
	st, err := site.Build(m)
	if err != nil {
		t.Fatal(err)
	}
	var reported []string
	panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Items []struct{ Path string } }
		_ = json.NewDecoder(r.Body).Decode(&body)
		for _, it := range body.Items {
			reported = append(reported, it.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer panel.Close()
	rep := &unmapped.Reporter{URL: panel.URL, Client: panel.Client()}
	sealer, _ := formtoken.New(formtoken.RandomKey())
	mgr, _ := session.NewManager(session.NewMemory(), formtoken.RandomKey())
	p := &Public{Maps: fixedMaps{st}, IPs: clientip.New(nil), Forms: sealer, Sessions: mgr, Unmapped: rep,
		Log: slog.New(slog.NewJSONHandler(io.Discard, nil)), Events: telemetry.NewEmitter(io.Discard, "test")}

	for _, c := range []struct {
		target string
		code   int
		legacy string // what the legacy site gets, "" none
	}{
		{"/?p=5&zz=1", 200, "/?p=5&zz=1"},                             // an extra key: the area, as before
		{"/?p=1%27+OR+%271%27%3D%271&zz=1", 400, ""},                  // ...but the declared key is still checked
		{"/?zz=%3Cscript%3E", 200, "/?zz=%3Cscript%3E"},               // a key no entry declares: the area's
		{"/1%27OR%271%27%3D%271/", 400, ""},                           // a path value no page has
		{"/%3Cscript%3E/", 400, ""},                                   //
		{"/wp-admin/", 200, "/wp-admin/"},                             // a name the type denies: the area
		{"/citt%C3%A0/", 200, "/citt%C3%A0/"},                         // accents fit
		{"/wp-includes/js/x.js?ver=6.9", 200, "/wp-includes/js/x.js"}, // a file, not a dated post
	} {
		mu.Lock()
		hits = nil
		mu.Unlock()
		req := httptest.NewRequest("GET", "http://blog.example"+c.target, nil)
		req.Host = "blog.example"
		rec := httptest.NewRecorder()
		p.Handler().ServeHTTP(rec, req)
		mu.Lock()
		got := append([]string(nil), hits...)
		mu.Unlock()
		if rec.Code != c.code {
			t.Errorf("%s: %d, want %d", c.target, rec.Code, c.code)
		}
		switch {
		case c.legacy == "" && len(got) > 0:
			t.Errorf("%s reached the legacy site: %v", c.target, got)
		case c.legacy != "" && (len(got) != 1 || got[0] != c.legacy):
			t.Errorf("%s: the legacy site got %v, want %s", c.target, got, c.legacy)
		}
	}
	if err := rep.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	// the key that went on to the area is reported for the discovery, never a value
	found := false
	for _, r := range reported {
		found = found || r == "/?zz="
		if r == "/?zz=%3Cscript%3E" || r == "/?zz=<script>" {
			t.Errorf("a value was reported: %q", r)
		}
	}
	if !found {
		t.Errorf("/?zz= not reported: %v", reported)
	}
}
