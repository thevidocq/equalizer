package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/site"
)

func apiSite(t *testing.T, legacy http.HandlerFunc) *site.Site {
	t.Helper()
	srv := httptest.NewServer(legacy)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	no := false
	m := &mapmodel.Model{
		Meta:   mapmodel.Meta{Tenant: "t", App: "a", Version: 1, Status: "published"},
		Legacy: mapmodel.Legacy{BaseURL: srv.URL, Reach: mapmodel.Reach{Mode: "direct", Allow: []string{u.Host}}},
		Roles:  mapmodel.Roles{Anonymous: "guest", List: map[string]any{"guest": nil}},
		API: []mapmodel.APIRoute{
			{ID: "list", Path: "/api/permits", Method: "GET", Roles: []string{"guest"},
				Query:         map[string]mapmodel.FieldSpec{"page": {Type: "int", Required: &no}},
				LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/rest/permits", Query: map[string]string{"p": "{{query.page}}"}},
				Response: mapmodel.APIResponse{List: true, Path: "items", Fields: map[string]mapmodel.FieldSpec{
					"id": {Type: "int"}, "title": {Type: "string"}}}},
			{ID: "save", Path: "/api/permits/{id}", Method: "PUT", Roles: []string{"guest"},
				Params:        map[string]mapmodel.FieldSpec{"id": {Type: "int"}},
				Body:          map[string]mapmodel.FieldSpec{"title": {Type: "string", MaxLength: 50}, "urgent": {Type: "bool", Required: &no}},
				LegacyRequest: mapmodel.LegacyRequest{Method: "PUT", Path: "/rest/permits/{{params.id}}", JSON: map[string]any{"title": "{{body.title}}", "urgent": "{{body.urgent}}", "source": "web"}},
				Response:      mapmodel.APIResponse{Fields: map[string]mapmodel.FieldSpec{"id": {Type: "int"}, "title": {Type: "string"}}}},
		},
	}
	st, err := site.Build(m)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestAPI(t *testing.T) {
	var seen *http.Request
	var seenBody string
	st := apiSite(t, func(w http.ResponseWriter, r *http.Request) {
		seen = r
		b, _ := io.ReadAll(r.Body)
		seenBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Set-Cookie", "JSESSIONID=abc")
		switch {
		case r.URL.Path == "/rest/permits":
			_, _ = io.WriteString(w, `{"items":[{"id":1,"title":"Shed","owner":"RSSMRA80A01H501U"}],"sql":"SELECT *"}`)
		case r.URL.Path == "/rest/permits/9":
			w.WriteHeader(500)
			_, _ = io.WriteString(w, `{"error":"java.lang.NullPointerException at ..."}`)
		default:
			_, _ = io.WriteString(w, `{"id":5,"title":"Barn","internal_flag":true}`)
		}
	})
	h := newPublic(t, st, io.Discard)
	call := func(method, target, body string, hdr map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, target, strings.NewReader(body))
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	jsonHdr := map[string]string{"Content-Type": "application/json"}

	rec := call("GET", "/api/permits?page=2", "", nil)
	if rec.Code != 200 || rec.Body.String() != `[{"id":1,"title":"Shed"}]` || seen.URL.RawQuery != "p=2" {
		t.Fatalf("list: %d %s (legacy query %q)", rec.Code, rec.Body.String(), seen.URL.RawQuery)
	}
	if rec.Header().Get("Content-Type") != "application/json; charset=utf-8" || rec.Header().Get("Set-Cookie") != "" {
		t.Errorf("list headers: %v", rec.Header())
	}
	if call("GET", "/api/permits?page=x", "", nil).Code != 400 {
		t.Error("bad query accepted")
	}

	rec = call("PUT", "/api/permits/5", `{"title":"Barn","urgent":true}`, jsonHdr)
	if rec.Code != 200 || rec.Body.String() != `{"id":5,"title":"Barn"}` {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	var sent map[string]any
	_ = json.Unmarshal([]byte(seenBody), &sent)
	if seen.Method != "PUT" || seen.Header.Get("Content-Type") != "application/json" || sent["urgent"] != true || sent["title"] != "Barn" || sent["source"] != "web" {
		t.Fatalf("legacy got %s %v %s", seen.Method, seen.Header, seenBody)
	}

	for name, c := range map[string]struct {
		method, target, body string
		hdr                  map[string]string
		status               int
	}{
		"extra field":       {"PUT", "/api/permits/5", `{"title":"a","role":"admin"}`, jsonHdr, 400},
		"wrong type":        {"PUT", "/api/permits/5", `{"title":"a","urgent":"yes"}`, jsonHdr, 400},
		"not json":          {"PUT", "/api/permits/5", `title=a`, map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, 400},
		"other site":        {"PUT", "/api/permits/5", `{"title":"a"}`, map[string]string{"Content-Type": "application/json", "Origin": "https://evil.example"}, 403},
		"cross-site fetch":  {"PUT", "/api/permits/5", `{"title":"a"}`, map[string]string{"Content-Type": "application/json", "Sec-Fetch-Site": "cross-site"}, 403},
		"bad path param":    {"PUT", "/api/permits/abc", `{"title":"a"}`, jsonHdr, 400},
		"method not mapped": {"DELETE", "/api/permits/5", ``, nil, 405},
	} {
		if rec := call(c.method, c.target, c.body, c.hdr); rec.Code != c.status {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
	}

	// A legacy error never shows its body.
	rec = call("PUT", "/api/permits/9", `{"title":"a"}`, jsonHdr)
	if rec.Code != 502 || bytes.Contains(rec.Body.Bytes(), []byte("java")) {
		t.Fatalf("legacy error leaked: %d %s", rec.Code, rec.Body.String())
	}
}
