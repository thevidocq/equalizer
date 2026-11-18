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
			{ID: "elenco", Path: "/pratiche",
				Query:         map[string]mapmodel.FieldSpec{"stato": {Type: "string"}, "pagina": {Type: "int"}},
				LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/pratiche/list.jsp", Query: map[string]string{"st": "{{query.stato}}", "p": "{{query.pagina}}"}}},
			{ID: "dettaglio", Path: "/pratiche/{id}",
				Params:        map[string]mapmodel.FieldSpec{"id": {Type: "int"}},
				LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/pratiche/detail.jsp", Query: map[string]string{"id": "{{params.id}}", "v": "full"}},
				Links:         []mapmodel.Link{{Legacy: "/vecchio/scheda.asp?cod={id}", Route: "dettaglio"}}},
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
		{"http://10.20.0.15:8080/app/pratiche/detail.jsp?id=42&v=full", "/pratiche/42", true},
		{"http://10.20.0.15:8080/app/pratiche/detail.jsp?v=full&id=42#allegati", "/pratiche/42#allegati", true},
		{"http://10.20.0.15:8080/app/pratiche/list.jsp?st=aperta", "/pratiche?stato=aperta", true},
		{"http://10.20.0.15:8080/app/pratiche/list.jsp", "/pratiche", true},
		{"http://10.20.0.15:8080/app/vecchio/scheda.asp?cod=7", "/pratiche/7", true},
		{"http://10.20.0.15:8080/app/Default.aspx", "/", true},
		{"http://10.20.0.15:8080/app/css/stile.css?v=3", "/css/stile.css?v=3", true},
		{"http://10.20.0.15:8080/app/pratiche/detail.jsp?id=42", "", false},            // manca v=full
		{"http://10.20.0.15:8080/app/pratiche/detail.jsp?id=42&v=full&x=1", "", false}, // chiave in più
		{"http://10.20.0.15:8080/app/admin/console.jsp", "", false},
		{"http://10.20.0.15:8080/altro/index.jsp", "", false}, // fuori dal percorso base
	}
	for _, c := range cases {
		u, _ := url.Parse(c.legacy)
		got, ok := r.Public(u)
		if ok != c.ok || got != c.want {
			t.Errorf("%s → (%q, %v), atteso (%q, %v)", c.legacy, got, ok, c.want, c.ok)
		}
	}
}

func TestInternalAndFallback(t *testing.T) {
	base, _ := url.Parse("http://10.20.0.15:8080/")
	r, _ := New(base, testModel())
	in, _ := url.Parse("http://10.20.0.15:8080/admin/x.jsp?a=1")
	out, _ := url.Parse("https://www.pagopa.gov.it/")
	if !r.Internal(in) || r.Internal(out) {
		t.Fatal("riconoscimento interno/esterno errato")
	}
	if got := r.Fallback(in); got != "/admin/x.jsp?a=1" {
		t.Fatalf("fallback %q: l'host interno non deve comparire", got)
	}
}

func TestRejectsBadLinks(t *testing.T) {
	m := testModel()
	m.Routes[2].Links = []mapmodel.Link{{Legacy: "/x?cod={sconosciuto}", Route: "dettaglio"}}
	base, _ := url.Parse("http://h/")
	if _, err := New(base, m); err == nil {
		t.Fatal("link con segnaposto sconosciuto accettato")
	}
}
