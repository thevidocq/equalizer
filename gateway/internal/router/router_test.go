package router

import (
	"errors"
	"testing"
)

func testRouter(t *testing.T) *Router {
	t.Helper()
	r, err := New([]Entry{
		{ID: "home", Kind: KindRoute, Method: "GET", Pattern: "/"},
		{ID: "elenco", Kind: KindRoute, Method: "GET", Pattern: "/pratiche"},
		{ID: "dettaglio", Kind: KindRoute, Method: "GET", Pattern: "/pratiche/{id}"},
		{ID: "nuova", Kind: KindRoute, Method: "GET", Pattern: "/pratiche/nuova"},
		{ID: "integrazione", Kind: KindForm, Method: "POST", Pattern: "/pratiche/{id}/integrazione"},
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
		{"GET", "/pratiche", "elenco", ""},
		{"GET", "/pratiche/", "elenco", ""},
		{"GET", "/pratiche/42", "dettaglio", "42"},
		{"GET", "/pratiche/nuova", "nuova", ""}, // il letterale vince sul parametro
		{"POST", "/pratiche/42/integrazione", "integrazione", "42"},
	}
	for _, c := range cases {
		m, err := r.Match(c.method, c.path)
		if err != nil {
			t.Fatalf("%s %s: errore %v", c.method, c.path, err)
		}
		if m.Entry.ID != c.want {
			t.Errorf("%s %s: voce %q, attesa %q", c.method, c.path, m.Entry.ID, c.want)
		}
		if c.param != "" && m.Params["id"] != c.param {
			t.Errorf("%s %s: id %q, atteso %q", c.method, c.path, m.Params["id"], c.param)
		}
	}
}

func TestDenyByDefault(t *testing.T) {
	r := testRouter(t)
	for _, p := range []string{"/wp-admin", "/pratiche/42/altro", "/manager/html", "/pratiche//integrazione"} {
		if _, err := r.Match("GET", p); !errors.Is(err, ErrNotMapped) && !errors.Is(err, ErrMethodNotAllowed) {
			t.Errorf("GET %s: atteso blocco, ottenuto %v", p, err)
		}
	}
	if _, err := r.Match("GET", "/wp-admin"); !errors.Is(err, ErrNotMapped) {
		t.Errorf("GET /wp-admin: atteso ErrNotMapped, ottenuto %v", err)
	}
	if _, err := r.Match("POST", "/pratiche"); !errors.Is(err, ErrMethodNotAllowed) {
		t.Errorf("POST /pratiche: atteso ErrMethodNotAllowed, ottenuto %v", err)
	}
	if _, err := r.Match("DELETE", "/pratiche/42"); !errors.Is(err, ErrMethodNotAllowed) {
		t.Errorf("DELETE /pratiche/42: atteso ErrMethodNotAllowed, ottenuto %v", err)
	}
}

func TestNewRejectsConflicts(t *testing.T) {
	_, err := New([]Entry{
		{ID: "a", Method: "GET", Pattern: "/x/{id}"},
		{ID: "b", Method: "GET", Pattern: "/x/{codice}"},
	})
	if err == nil {
		t.Fatal("attesa voce in conflitto")
	}
	if _, err := New([]Entry{{ID: "c", Method: "GET", Pattern: "x"}}); err == nil {
		t.Fatal("atteso errore per percorso senza /")
	}
	if _, err := New([]Entry{{ID: "d", Method: "GET", Pattern: "/a{b}"}}); err == nil {
		t.Fatal("atteso errore per segmento malformato")
	}
}
