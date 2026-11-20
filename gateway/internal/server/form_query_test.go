package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/site"
)

// A login form whose action carries the legacy session id in its query
// (phpBB's ucp.php?mode=login&sid=…): bound like any form, the id never
// reaches the page, and the legacy site gets it back on submission.
func TestFormActionQueryKeptServerSide(t *testing.T) {
	var gotQuery url.Values
	legacy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.Method == http.MethodPost {
			gotQuery = r.URL.Query()
			_, _ = io.WriteString(w, "<p>Welcome</p>")
			return
		}
		_, _ = io.WriteString(w, `<form method="post" action="./ucp.php?mode=login&amp;sid=s3cr3t">`+
			`<input name="username"><button>Login</button></form>`)
	})
	srv := httptest.NewServer(legacy)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	m := &mapmodel.Model{
		Meta:   mapmodel.Meta{Tenant: "t", App: "a", Version: 1},
		Legacy: mapmodel.Legacy{BaseURL: srv.URL, Reach: mapmodel.Reach{Mode: "direct", Allow: []string{u.Host}}},
		Roles:  mapmodel.Roles{Anonymous: "guest", List: map[string]any{"guest": nil}},
		Routes: []mapmodel.Route{{ID: "home", Path: "/", Roles: []string{"guest"},
			LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/index.php"}}},
		Forms: []mapmodel.Form{{ID: "login", Route: "/ucp.php", Roles: []string{"guest"},
			Fields: map[string]mapmodel.FieldSpec{"username": {Type: "string", MaxLength: 40, Charset: "[a-z]"}},
			LegacyRequest: mapmodel.LegacyRequest{Method: "POST", Path: "/ucp.php",
				Query: map[string]string{"mode": "login", "sid": "{{hidden.sid}}"},
				Form:  map[string]string{"username": "{{fields.username}}"}}}},
	}
	st, err := site.Build(m)
	if err != nil {
		t.Fatal(err)
	}
	h := formPublic(t, st, io.Discard)
	rec := get(h, "GET", "/", nil)
	body := rec.Body.String()
	if strings.Contains(body, "s3cr3t") || !strings.Contains(body, `action="/ucp.php"`) {
		t.Fatalf("page: %s", body)
	}
	token := tokenRe.FindStringSubmatch(body)
	if token == nil || len(rec.Result().Cookies()) != 1 {
		t.Fatalf("form not bound: %s", body)
	}
	rec = post(h, "/ucp.php", url.Values{"username": {"editor"}, "_eq_token": {token[1]}},
		withCookie(rec.Result().Cookies()[0].Value))
	if rec.Code != 200 || gotQuery.Get("sid") != "s3cr3t" || gotQuery.Get("mode") != "login" {
		t.Fatalf("submission: %d %v %s", rec.Code, gotQuery, rec.Body.String())
	}
}
