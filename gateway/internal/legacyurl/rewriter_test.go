package legacyurl

import (
	"net/url"
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
)

func testModel() *mapmodel.Model {
	return &mapmodel.Model{
		Routes: []mapmodel.Route{
			{ID: "home", Path: "/", LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/index.jsp"}},
			{ID: "list", Path: "/permits",
				Query:         map[string]mapmodel.FieldSpec{"status": {Type: "string"}, "page": {Type: "int"}},
				LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/permits/list.jsp", Query: map[string]string{"st": "{{query.status}}", "p": "{{query.page}}"}}},
			{ID: "detail", Path: "/permits/{id}",
				Params:        map[string]mapmodel.FieldSpec{"id": {Type: "int"}},
				LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/permits/detail.jsp", Query: map[string]string{"id": "{{params.id}}", "v": "full"}},
				Links:         []mapmodel.Link{{Legacy: "/old/record.asp?code={id}", Route: "detail"}}},
		},
		Compat: []mapmodel.Compat{{Legacy: "/Default.aspx", Route: "home"}},
		Assets: []mapmodel.Asset{{Prefix: "/css/", Types: []string{"text/css"}}},
	}
}

func TestPublic(t *testing.T) {
	base, _ := url.Parse("http://10.20.0.15:8080/app")
	r, err := New(base, testModel())
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		legacy, want string
		ok           bool
	}{
		{"http://10.20.0.15:8080/app/index.jsp", "/", true},
		{"http://10.20.0.15:8080/app/permits/detail.jsp?id=42&v=full", "/permits/42", true},
		{"http://10.20.0.15:8080/app/permits/detail.jsp?v=full&id=42#files", "/permits/42#files", true},
		{"http://10.20.0.15:8080/app/permits/list.jsp?st=open", "/permits?status=open", true},
		{"http://10.20.0.15:8080/app/permits/list.jsp", "/permits", true},
		{"http://10.20.0.15:8080/app/old/record.asp?code=7", "/permits/7", true},
		{"http://10.20.0.15:8080/app/Default.aspx", "/", true},
		{"http://10.20.0.15:8080/app/css/style.css?v=3", "/css/style.css?v=3", true},
		{"http://10.20.0.15:8080/app/permits/detail.jsp?id=42", "", false},            // v=full missing
		{"http://10.20.0.15:8080/app/permits/detail.jsp?id=42&v=full&x=1", "", false}, // extra key
		{"http://10.20.0.15:8080/app/admin/console.jsp", "", false},
		{"http://10.20.0.15:8080/other/index.jsp", "", false}, // outside the base path
	}
	for _, c := range cases {
		u, _ := url.Parse(c.legacy)
		got, ok := r.Public(u)
		if ok != c.ok || got != c.want {
			t.Errorf("%s → (%q, %v), want (%q, %v)", c.legacy, got, ok, c.want, c.ok)
		}
	}
}

func TestInternalAndFallback(t *testing.T) {
	base, _ := url.Parse("http://10.20.0.15:8080/")
	r, _ := New(base, testModel())
	in, _ := url.Parse("http://10.20.0.15:8080/admin/x.jsp?a=1")
	out, _ := url.Parse("https://www.example.org/")
	if !r.Internal(in) || r.Internal(out) {
		t.Fatal("wrong internal/external detection")
	}
	if got := r.Fallback(in); got != "/admin/x.jsp?a=1" {
		t.Fatalf("fallback %q: the internal host must not appear", got)
	}
}

func TestRejectsBadLinks(t *testing.T) {
	m := testModel()
	m.Routes[2].Links = []mapmodel.Link{{Legacy: "/x?code={unknown}", Route: "detail"}}
	base, _ := url.Parse("http://h/")
	if _, err := New(base, m); err == nil {
		t.Fatal("link with an unknown placeholder accepted")
	}
}

func TestTypesAndTrailingSlash(t *testing.T) {
	base, _ := url.Parse("http://legacy.example")
	m := &mapmodel.Model{
		Areas: []mapmodel.Area{{ID: "site", Prefix: "/"}},
		Routes: []mapmodel.Route{{ID: "page", Path: "/{slug}/", Method: "GET",
			Params:        map[string]mapmodel.FieldSpec{"slug": {Type: "string"}},
			LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/{{params.slug}}/"}}},
	}
	r, err := New(base, m)
	if err != nil {
		t.Fatal(err)
	}
	r.Fits = func(_ *mapmodel.Route, p map[string]string) bool { return p["slug"] != "wp-admin" }
	for in, want := range map[string]string{
		"http://legacy.example/about/":    "/about/",
		"http://legacy.example/wp-admin/": "/wp-admin/", // not a page: the area, as it is
	} {
		u, _ := url.Parse(in)
		if got, ok := r.Public(u); !ok || got != want {
			t.Errorf("%s: %q %v", in, got, ok)
		}
	}
}

// A page's own escaping stays: WordPress writes %ce%b5, Go would write
// %CE%B5 (the same address, but not the page the site sent).
func TestKeepsTheSitesEscaping(t *testing.T) {
	base, _ := url.Parse("http://10.0.0.1:8080/")
	m := &mapmodel.Model{
		Routes: []mapmodel.Route{{ID: "tag", Path: "/tag/{slug}/", Params: map[string]mapmodel.FieldSpec{"slug": {Type: "string"}},
			LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/tag/{{params.slug}}/"}}},
		Assets: []mapmodel.Asset{{Prefix: "/wp-content/uploads/", Types: []string{"image/jpeg"}}},
		Areas:  []mapmodel.Area{{Prefix: "/"}},
	}
	r, err := New(base, m)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []string{
		"/tag/%ce%b5%cf%80%ce%af/",             // a route
		"/wp-content/uploads/%ce%b5%cf%80.jpg", // a file
		"/category/%ce%b5%cf%80/?paged=2#top",  // the area
		"/tag/%CE%B5%CF%80%CE%AF/",             // already upper case: unchanged too
	} {
		u, _ := url.Parse("http://10.0.0.1:8080" + c)
		if got, ok := r.Public(u); !ok || got != c {
			t.Errorf("%s → %q, %v", c, got, ok)
		}
	}
}

func TestEscapingUnderABasePathAndOtherHosts(t *testing.T) {
	base, _ := url.Parse("http://10.0.0.1:8080/blog/")
	m := &mapmodel.Model{
		Assets: []mapmodel.Asset{{Prefix: "/wp-content/", Types: []string{"image/jpeg"}}},
		Areas:  []mapmodel.Area{{Prefix: "/"}},
	}
	r, err := New(base, m)
	if err != nil {
		t.Fatal(err)
	}
	for in, want := range map[string]string{
		"/blog/tag/%ce%b5/":           "/tag/%ce%b5/",
		"/blog/wp-content/%ce%b5.jpg": "/wp-content/%ce%b5.jpg",
		"/blog//evil.test/x":          "/.//evil.test/x", // never another host
		"/blog/%2Fevil.test":          "/%2Fevil.test",
	} {
		u, _ := url.Parse("http://10.0.0.1:8080" + in)
		if got, ok := r.Public(u); !ok || got != want {
			t.Errorf("%s → %q, %v; want %q", in, got, ok, want)
		}
	}
	u, _ := url.Parse("http://10.0.0.1:8080//evil.test/x")
	if got := r.Fallback(u); got != "/.//evil.test/x" {
		t.Errorf("fallback %q", got)
	}
}
