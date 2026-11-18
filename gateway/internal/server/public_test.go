package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/clientip"
	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
)

type fixedMaps struct{ s *site.Site }

func (f fixedMaps) Current() *site.Site { return f.s }

func fp(v float64) *float64 { return &v }

// testSite costruisce un sito con un legacy finto in httptest.
func testSite(t *testing.T, legacy http.Handler) *site.Site {
	t.Helper()
	srv := httptest.NewServer(legacy)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	m := &mapmodel.Model{
		Meta:   mapmodel.Meta{Tenant: "t", App: "a", Version: 7, Status: "published"},
		Legacy: mapmodel.Legacy{BaseURL: srv.URL, Reach: mapmodel.Reach{Mode: "direct", Allow: []string{u.Host}}},
		Roles:  mapmodel.Roles{Anonymous: "anonimo", List: map[string]any{"anonimo": nil, "cittadino": nil}},
		Types:  map[string]mapmodel.TypeDef{"id_pratica": {Base: "int", Min: fp(1), Max: fp(1000)}},
		Routes: []mapmodel.Route{
			{ID: "home", Path: "/", Method: "GET", Roles: []string{"anonimo"},
				LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/index.jsp"}},
			{ID: "dettaglio", Path: "/pratiche/{id}", Method: "GET", Roles: []string{"anonimo"},
				Params:        map[string]mapmodel.FieldSpec{"id": {Type: "id_pratica"}},
				Query:         map[string]mapmodel.FieldSpec{"tab": {Type: "enum", Values: []string{"dati", "allegati"}, Required: new(bool)}},
				LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/detail.jsp", Query: map[string]string{"id": "{{params.id}}", "t": "{{query.tab}}"}}},
			{ID: "riservata", Path: "/area", Method: "GET", Roles: []string{"cittadino"},
				LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/area.jsp"}},
		},
	}
	st, err := site.Build(m)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func newPublic(t *testing.T, st *site.Site, events io.Writer) http.Handler {
	t.Helper()
	p := &Public{
		Maps:   fixedMaps{st},
		IPs:    clientip.New(nil),
		Log:    slog.New(slog.NewJSONHandler(io.Discard, nil)),
		Events: telemetry.NewEmitter(events, "test"),
	}
	return p.Handler()
}

func get(h http.Handler, method, target string, header http.Header) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	for k, v := range header {
		req.Header[k] = v
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func lastEvent(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	var ev map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &ev); err != nil {
		t.Fatalf("evento non JSON: %v", err)
	}
	return ev
}

func TestReconstructsRequest(t *testing.T) {
	var seen *http.Request
	st := testSite(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r
		w.Header().Set("Server", "Apache-Coyote/1.1")
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<p>pratica</p>"))
	}))
	var events bytes.Buffer
	h := newPublic(t, st, &events)

	rec := get(h, "GET", "/pratiche/0042?tab=allegati&debug=1&id=999", http.Header{
		"Cookie":          {"admin=1"},
		"X-Forwarded-For": {"6.6.6.6"},
		"Authorization":   {"Basic YWRtaW46YWRtaW4="},
	})
	if rec.Code != http.StatusOK || rec.Body.String() != "<p>pratica</p>" {
		t.Fatalf("risposta %d %q", rec.Code, rec.Body.String())
	}
	if seen.URL.Path != "/detail.jsp" || seen.URL.Query().Get("id") != "42" || seen.URL.Query().Get("t") != "allegati" {
		t.Fatalf("richiesta ricostruita male: %s?%s", seen.URL.Path, seen.URL.RawQuery)
	}
	if seen.URL.Query().Has("debug") || len(seen.URL.Query()) != 2 {
		t.Fatalf("parametri non dichiarati inoltrati: %q", seen.URL.RawQuery)
	}
	for _, h := range []string{"Cookie", "X-Forwarded-For", "Authorization"} {
		if seen.Header.Get(h) != "" {
			t.Errorf("header %s del client arrivato al legacy", h)
		}
	}
	if rec.Header().Get("Server") != "" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("header di risposta non filtrati: %v", rec.Header())
	}
	if ev := lastEvent(t, &events); ev["type"] != "request.served" || ev["route"] != "dettaglio" {
		t.Errorf("evento inatteso: %v", ev)
	}
}

func TestRejectsInvalidValues(t *testing.T) {
	calls := 0
	st := testSite(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	var events bytes.Buffer
	h := newPublic(t, st, &events)

	for _, target := range []string{
		"/pratiche/1%20OR%201=1",
		"/pratiche/5000",
		"/pratiche/42?tab=../../etc/passwd",
		"/pratiche/42?tab=dati&tab=allegati",
		"/pratiche/42?tab=%zz",
	} {
		rec := get(h, "GET", target, nil)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, atteso 400", target, rec.Code)
		}
		ev := lastEvent(t, &events)
		if ev["type"] != "field.rejected" {
			t.Errorf("%s: evento %v", target, ev["type"])
		}
		// Il percorso è registrato (serve a chi analizza gli attacchi),
		// i valori della query mai.
		if strings.Contains(events.String(), "passwd") {
			t.Fatal("un valore della query non deve finire negli eventi")
		}
	}
	if calls != 0 {
		t.Fatalf("il legacy è stato chiamato %d volte con valori non validi", calls)
	}
}

func TestRoleAndLegacyErrors(t *testing.T) {
	st := testSite(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/index.jsp" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("java.lang.NullPointerException at com.example.Foo"))
		}
	}))
	h := newPublic(t, st, io.Discard)

	if rec := get(h, "GET", "/area", nil); rec.Code != http.StatusForbidden {
		t.Errorf("rotta riservata: status %d, atteso 403", rec.Code)
	}
	rec := get(h, "GET", "/", nil)
	if rec.Code != http.StatusBadGateway || strings.Contains(rec.Body.String(), "java.lang") {
		t.Errorf("errore del legacy esposto: %d %q", rec.Code, rec.Body.String())
	}
}

func TestPublicBlocksUnmappedAndMethods(t *testing.T) {
	st := testSite(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	var events bytes.Buffer
	h := newPublic(t, st, &events)

	if rec := get(h, "GET", "/wp-admin?user=admin", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, atteso 404", rec.Code)
	}
	ev := lastEvent(t, &events)
	if ev["type"] != "request.blocked" || ev["reason"] != "route_not_mapped" || ev["map_version"] != float64(7) {
		t.Fatalf("evento inatteso: %v", ev)
	}
	if strings.Contains(events.String(), "user=admin") {
		t.Fatal("la query string non deve finire negli eventi")
	}
	cases := map[string]int{"TRACE": 405, "CONNECT": 405, "OPTIONS": 204, "POST": 405}
	for method, want := range cases {
		if rec := get(h, method, "/", nil); rec.Code != want {
			t.Errorf("%s /: status %d, atteso %d", method, rec.Code, want)
		}
	}
}

func TestPublicWithoutMapFailsClosed(t *testing.T) {
	h := newPublic(t, nil, io.Discard)
	if rec := get(h, "GET", "/", nil); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d, atteso 503", rec.Code)
	}
}
