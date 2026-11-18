package server

import (
	"bytes"
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

// The legacy site lists permits 5 and 7 for this visitor; 9 exists too but
// is someone else's. Changing the URL to 9 must not work.
func TestOwnedByUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/list.jsp":
			_, _ = io.WriteString(w, `<a href="/detail.jsp?id=5">5</a> <a href="/detail.jsp?id=7">7</a>`)
		case "/new.jsp":
			http.Redirect(w, r, "/detail.jsp?id=11", http.StatusFound)
		default:
			_, _ = io.WriteString(w, "<p>permit "+r.URL.Query().Get("id")+"</p>")
		}
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	no := false
	m := &mapmodel.Model{
		Meta:   mapmodel.Meta{Tenant: "t", App: "a", Version: 1, Status: "published"},
		Legacy: mapmodel.Legacy{BaseURL: srv.URL, Reach: mapmodel.Reach{Mode: "direct", Allow: []string{u.Host}}},
		Roles:  mapmodel.Roles{Anonymous: "guest", List: map[string]any{"guest": nil}},
		Routes: []mapmodel.Route{
			{ID: "list", Path: "/permits", Method: "GET", Roles: []string{"guest"}, LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/list.jsp"}},
			{ID: "new", Path: "/permits/new", Method: "GET", Roles: []string{"guest"}, LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/new.jsp"}},
			{ID: "detail", Path: "/permits/{id}", Method: "GET", Roles: []string{"guest"},
				Params:        map[string]mapmodel.FieldSpec{"id": {Type: "int", Required: &no}},
				LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/detail.jsp", Query: map[string]string{"id": "{{params.id}}"}},
				Authz:         map[string]mapmodel.AuthzRule{"guest": {Check: "owned_by_user", Target: "params.id"}}},
		},
	}
	st, err := site.Build(m)
	if err != nil {
		t.Fatal(err)
	}
	sealer, _ := formtoken.New(formtoken.RandomKey())
	mgr, _ := session.NewManager(session.NewMemory(), formtoken.RandomKey())
	var events bytes.Buffer
	p := &Public{Maps: fixedMaps{st}, IPs: clientip.New(nil), Forms: sealer, Sessions: mgr,
		Log: slog.New(slog.NewJSONHandler(io.Discard, nil)), Events: telemetry.NewEmitter(&events, "test")}
	b := &browser{t: t, h: p.Handler(), cookies: map[string]string{}}

	if rec := b.do("GET", "/permits/5", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("before the list: %d", rec.Code)
	}
	rec := b.do("GET", "/permits", nil)
	if !strings.Contains(rec.Body.String(), `href="/permits/5"`) {
		t.Fatalf("list not rewritten: %s", rec.Body.String())
	}
	for path, want := range map[string]int{"/permits/5": 200, "/permits/7": 200, "/permits/9": 403} {
		if rec := b.do("GET", path, nil); rec.Code != want {
			t.Errorf("%s: %d, want %d", path, rec.Code, want)
		}
	}
	// A redirect issues its target too (an object just created).
	if rec := b.do("GET", "/permits/new", nil); rec.Code != http.StatusFound || rec.Header().Get("Location") != "/permits/11" {
		t.Fatalf("redirect: %d %v", rec.Code, rec.Header())
	}
	if rec := b.do("GET", "/permits/11", nil); rec.Code != 200 {
		t.Errorf("created object: %d", rec.Code)
	}
	// Another browser has seen nothing.
	other := &browser{t: t, h: p.Handler(), cookies: map[string]string{}}
	if rec := other.do("GET", "/permits/5", nil); rec.Code != http.StatusForbidden {
		t.Errorf("another session: %d", rec.Code)
	}
	if !strings.Contains(events.String(), "authz_not_owned") {
		t.Error("the refusal is not in the events")
	}

	bad := *m
	bad.Routes = append([]mapmodel.Route{}, m.Routes...)
	bad.Routes[2].Authz = map[string]mapmodel.AuthzRule{"guest": {Check: "owned_by_user", Target: "params.nope"}}
	if _, err := site.Build(&bad); err == nil {
		t.Error("a target that is not a declared value was accepted")
	}
}

// A single-page app: the list comes as JSON, then the browser asks for one
// permit by id. The ids of the JSON list are issued like links of a page.
func TestOwnedFromJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/rest/permits" {
			_, _ = io.WriteString(w, `{"items":[{"id":5,"title":"Shed"},{"id":7,"title":"Porch"}]}`)
			return
		}
		_, _ = io.WriteString(w, `{"id":1,"title":"x"}`)
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	m := &mapmodel.Model{
		Meta:   mapmodel.Meta{Tenant: "t", App: "a", Version: 1, Status: "published"},
		Legacy: mapmodel.Legacy{BaseURL: srv.URL, Reach: mapmodel.Reach{Mode: "direct", Allow: []string{u.Host}}},
		Roles:  mapmodel.Roles{Anonymous: "guest", List: map[string]any{"guest": nil}},
		API: []mapmodel.APIRoute{
			{ID: "list", Path: "/api/permits", Method: "GET", Roles: []string{"guest"},
				LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/rest/permits"},
				Response: mapmodel.APIResponse{List: true, Path: "items",
					Fields: map[string]mapmodel.FieldSpec{"id": {Type: "int"}, "title": {Type: "string"}},
					Issues: map[string]string{"id": "get.params.id"}}},
			{ID: "get", Path: "/api/permits/{id}", Method: "GET", Roles: []string{"guest"},
				Params:        map[string]mapmodel.FieldSpec{"id": {Type: "int"}},
				LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/rest/permits/{{params.id}}"},
				Response:      mapmodel.APIResponse{Fields: map[string]mapmodel.FieldSpec{"id": {Type: "int"}, "title": {Type: "string"}}},
				Authz:         map[string]mapmodel.AuthzRule{"guest": {Check: "owned_by_user", Target: "params.id"}}},
		},
	}
	st, err := site.Build(m)
	if err != nil {
		t.Fatal(err)
	}
	sealer, _ := formtoken.New(formtoken.RandomKey())
	mgr, _ := session.NewManager(session.NewMemory(), formtoken.RandomKey())
	p := &Public{Maps: fixedMaps{st}, IPs: clientip.New(nil), Forms: sealer, Sessions: mgr,
		Log: slog.New(slog.NewJSONHandler(io.Discard, nil)), Events: telemetry.NewEmitter(io.Discard, "test")}
	b := &browser{t: t, h: p.Handler(), cookies: map[string]string{}}

	if rec := b.do("GET", "/api/permits/5", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("before the list: %d", rec.Code)
	}
	if rec := b.do("GET", "/api/permits", nil); rec.Code != 200 {
		t.Fatalf("list: %d", rec.Code)
	}
	for path, want := range map[string]int{"/api/permits/5": 200, "/api/permits/7": 200, "/api/permits/9": 403} {
		if rec := b.do("GET", path, nil); rec.Code != want {
			t.Errorf("%s: %d, want %d", path, rec.Code, want)
		}
	}

	for name, issues := range map[string]map[string]string{
		"undeclared field": {"owner": "get.params.id"},
		"no rule there":    {"id": "list.params.id"},
		"wrong target":     {"id": "get.query.id"},
	} {
		bad := *m
		bad.API = append([]mapmodel.APIRoute{}, m.API...)
		bad.API[0].Response.Issues = issues
		if _, err := site.Build(&bad); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
