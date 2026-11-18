package router

import (
	"errors"
	"testing"
)

func testRouter(t *testing.T) *Router {
	t.Helper()
	r, err := New([]Entry{
		{ID: "home", Kind: KindRoute, Method: "GET", Pattern: "/"},
		{ID: "list", Kind: KindRoute, Method: "GET", Pattern: "/permits"},
		{ID: "detail", Kind: KindRoute, Method: "GET", Pattern: "/permits/{id}"},
		{ID: "new", Kind: KindRoute, Method: "GET", Pattern: "/permits/new"},
		{ID: "attach", Kind: KindForm, Method: "POST", Pattern: "/permits/{id}/attach"},
		{ID: "asset:/css/", Kind: KindAsset, Method: "GET", Pattern: "/css/"},
		{ID: "asset:/css/themes/", Kind: KindAsset, Method: "GET", Pattern: "/css/themes/"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestMatch(t *testing.T) {
	r := testRouter(t)
	cases := []struct {
		method, path, want, param string
	}{
		{"GET", "/", "home", ""},
		{"HEAD", "/", "home", ""},
		{"GET", "/permits", "list", ""},
		{"GET", "/permits/", "list", ""},
		{"GET", "/permits/42", "detail", "42"},
		{"GET", "/permits/new", "new", ""}, // the literal wins over the parameter
		{"POST", "/permits/42/attach", "attach", "42"},
	}
	for _, c := range cases {
		m, err := r.Match(c.method, c.path)
		if err != nil {
			t.Fatalf("%s %s: error %v", c.method, c.path, err)
		}
		if m.Entry.ID != c.want {
			t.Errorf("%s %s: entry %q, want %q", c.method, c.path, m.Entry.ID, c.want)
		}
		if c.param != "" && m.Params["id"] != c.param {
			t.Errorf("%s %s: id %q, want %q", c.method, c.path, m.Params["id"], c.param)
		}
	}
}

func TestDenyByDefault(t *testing.T) {
	r := testRouter(t)
	for _, p := range []string{"/wp-admin", "/permits/42/other", "/manager/html", "/permits//attach"} {
		if _, err := r.Match("GET", p); !errors.Is(err, ErrNotMapped) && !errors.Is(err, ErrMethodNotAllowed) {
			t.Errorf("GET %s: expected a block, got %v", p, err)
		}
	}
	if _, err := r.Match("GET", "/wp-admin"); !errors.Is(err, ErrNotMapped) {
		t.Errorf("GET /wp-admin: want ErrNotMapped, got %v", err)
	}
	if _, err := r.Match("POST", "/permits"); !errors.Is(err, ErrMethodNotAllowed) {
		t.Errorf("POST /permits: want ErrMethodNotAllowed, got %v", err)
	}
	if _, err := r.Match("DELETE", "/permits/42"); !errors.Is(err, ErrMethodNotAllowed) {
		t.Errorf("DELETE /permits/42: want ErrMethodNotAllowed, got %v", err)
	}
}

func TestAssets(t *testing.T) {
	r := testRouter(t)
	m, err := r.Match("GET", "/css/themes/blue.css")
	if err != nil || m.Entry.ID != "asset:/css/themes/" || m.Params["file"] != "blue.css" {
		t.Fatalf("longest prefix not chosen: %+v %v", m, err)
	}
	if _, err := r.Match("GET", "/css/"); !errors.Is(err, ErrNotMapped) {
		t.Fatalf("the prefix alone is not a file: %v", err)
	}
	if _, err := r.Match("POST", "/css/a.css"); !errors.Is(err, ErrMethodNotAllowed) {
		t.Fatalf("POST on an asset: %v", err)
	}
	if _, err := New([]Entry{{ID: "x", Kind: KindAsset, Pattern: "/a/../"}}); err == nil {
		t.Fatal("prefix with .. accepted")
	}
}

func TestNewRejectsConflicts(t *testing.T) {
	_, err := New([]Entry{
		{ID: "a", Method: "GET", Pattern: "/x/{id}"},
		{ID: "b", Method: "GET", Pattern: "/x/{code}"},
	})
	if err == nil {
		t.Fatal("expected conflicting entries")
	}
	if _, err := New([]Entry{{ID: "c", Method: "GET", Pattern: "x"}}); err == nil {
		t.Fatal("expected an error for a path without /")
	}
	if _, err := New([]Entry{{ID: "d", Method: "GET", Pattern: "/a{b}"}}); err == nil {
		t.Fatal("expected an error for a malformed segment")
	}
}
