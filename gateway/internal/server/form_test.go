package server

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/clientip"
	"github.com/thevidocq/equalizer/gateway/internal/formtoken"
	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
)

const detailPage = "<html><head><meta charset=\"iso-8859-1\"></head><body>" +
	"<form method=\"post\" action=\"supplement.do\">" +
	"<input type=\"hidden\" name=\"permitId\" value=\"42\">" +
	"<input type=\"hidden\" name=\"vs\" value=\"server-state\">" +
	"<input type=\"hidden\" name=\"debug\" value=\"1\">" +
	"<textarea name=\"text\"></textarea><button>Send</button></form></body></html>"

type formLegacy struct {
	posts []url.Values
	reply string
}

func (l *formLegacy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=iso-8859-1")
	switch r.URL.Path {
	case "/detail.jsp":
		_, _ = io.WriteString(w, detailPage)
	case "/supplement.do":
		raw, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(raw))
		l.posts = append(l.posts, vals)
		_, _ = io.WriteString(w, l.reply)
	case "/index.jsp":
		_, _ = io.WriteString(w, "<p>home</p>")
	}
}

func formSite(t *testing.T, legacy http.Handler) *site.Site {
	t.Helper()
	srv := httptest.NewServer(legacy)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	m := &mapmodel.Model{
		Meta:   mapmodel.Meta{Tenant: "t", App: "a", Version: 3},
		Legacy: mapmodel.Legacy{BaseURL: srv.URL, Reach: mapmodel.Reach{Mode: "direct", Allow: []string{u.Host}}},
		Roles:  mapmodel.Roles{Anonymous: "guest", List: map[string]any{"guest": nil}},
		Types:  map[string]mapmodel.TypeDef{"permit_id": {Base: "int", Min: fp(1), Max: fp(1000)}},
		Routes: []mapmodel.Route{
			{ID: "home", Path: "/", Roles: []string{"guest"},
				LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/index.jsp"}},
			{ID: "detail", Path: "/permits/{id}", Roles: []string{"guest"},
				Params:        map[string]mapmodel.FieldSpec{"id": {Type: "permit_id"}},
				LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/detail.jsp", Query: map[string]string{"id": "{{params.id}}"}}},
		},
		Forms: []mapmodel.Form{{
			ID: "supplement", Route: "/permits/{id}/supplement", Roles: []string{"guest"},
			Params: map[string]mapmodel.FieldSpec{"id": {Type: "permit_id"}},
			Fields: map[string]mapmodel.FieldSpec{"message": {Type: "text", MaxLength: 200, Rules: []string{"no_html"}}},
			LegacyRequest: mapmodel.LegacyRequest{Method: "POST", Path: "/supplement.do",
				Form: map[string]string{"permitId": "{{params.id}}", "text": "{{fields.message}}", "vs": "{{hidden.vs}}", "kind": "SUP"}},
			Success:   &mapmodel.SuccessCheck{BodyContains: "Supplement recorded"},
			OnSuccess: &mapmodel.OnSuccess{Redirect: "home"},
		}},
	}
	st, err := site.Build(m)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func formPublic(t *testing.T, st *site.Site, events io.Writer) http.Handler {
	t.Helper()
	sealer, err := formtoken.New(formtoken.RandomKey())
	if err != nil {
		t.Fatal(err)
	}
	p := &Public{Maps: fixedMaps{st}, IPs: clientip.New(nil), Forms: sealer,
		Log: slog.New(slog.NewJSONHandler(io.Discard, nil)), Events: telemetry.NewEmitter(events, "test")}
	return p.Handler()
}

var tokenRe = regexp.MustCompile(`name="_eq_token" value="([^"]+)"`)

// openForm loads the detail page and returns the form token and binding cookie.
func openForm(t *testing.T, h http.Handler) (string, string) {
	t.Helper()
	rec := get(h, "GET", "/permits/42", nil)
	body := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(body, `action="/permits/42/supplement"`) || !strings.Contains(body, `name="message"`) {
		t.Fatalf("form not bound: %d %s", rec.Code, body)
	}
	for _, banned := range []string{"server-state", "permitId", "debug", "supplement.do"} {
		if strings.Contains(body, banned) {
			t.Fatalf("%q reached the page: %s", banned, body)
		}
	}
	m := tokenRe.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no token in %s", body)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != bindingCookie || !cookies[0].Secure || !cookies[0].HttpOnly {
		t.Fatalf("binding cookie: %v", cookies)
	}
	return m[1], cookies[0].Value
}

func post(h http.Handler, target string, form url.Values, header http.Header) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for k, v := range header {
		req.Header[k] = v
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func withCookie(v string) http.Header {
	return http.Header{"Cookie": {bindingCookie + "=" + v}}
}

func TestFormSubmission(t *testing.T) {
	legacy := &formLegacy{reply: "<p>Supplement recorded</p>"}
	var events bytes.Buffer
	h := formPublic(t, formSite(t, legacy), &events)
	token, cookie := openForm(t, h)

	rec := post(h, "/permits/42/supplement", url.Values{
		"message": {"Café note"}, "_eq_token": {token}, "debug": {"1"}, "permitId": {"999"},
	}, withCookie(cookie))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("submission: %d %q %s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	if len(legacy.posts) != 1 {
		t.Fatalf("legacy POSTs: %d", len(legacy.posts))
	}
	got := legacy.posts[0]
	want := url.Values{"permitId": {"42"}, "text": {"Caf\xe9 note"}, "vs": {"server-state"}, "kind": {"SUP"}}
	if got.Encode() != want.Encode() {
		t.Errorf("rebuilt body %q, want %q", got.Encode(), want.Encode())
	}
	if ev := lastEvent(t, &events); ev["type"] != "request.served" || ev["route"] != "supplement" {
		t.Errorf("event: %v", ev)
	}
	if strings.Contains(events.String(), "note") {
		t.Error("field values must not end up in the events")
	}
}

func TestFormFailureShowsLegacyPage(t *testing.T) {
	legacy := &formLegacy{reply: "<p>Text too short</p>"}
	h := formPublic(t, formSite(t, legacy), io.Discard)
	token, cookie := openForm(t, h)
	rec := post(h, "/permits/42/supplement", url.Values{"message": {"x"}, "_eq_token": {token}}, withCookie(cookie))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Text too short") {
		t.Fatalf("failure page: %d %s", rec.Code, rec.Body.String())
	}
}

func TestFormRejections(t *testing.T) {
	legacy := &formLegacy{reply: "<p>Supplement recorded</p>"}
	var events bytes.Buffer
	h := formPublic(t, formSite(t, legacy), &events)
	token, cookie := openForm(t, h)
	_, otherCookie := openForm(t, h)
	ok := url.Values{"message": {"hello"}, "_eq_token": {token}}

	cases := []struct {
		name   string
		target string
		form   url.Values
		header http.Header
		status int
		reason string
	}{
		{"no cookie", "/permits/42/supplement", ok, nil, 403, "form_token_binding"},
		{"cookie of another browser", "/permits/42/supplement", ok, withCookie(otherCookie), 403, "form_token_binding"},
		{"no token", "/permits/42/supplement", url.Values{"message": {"hello"}}, withCookie(cookie), 403, "form_token_missing"},
		{"forged token", "/permits/42/supplement", url.Values{"message": {"hello"}, "_eq_token": {"AAAA" + token[4:]}}, withCookie(cookie), 403, "form_token_invalid"},
		{"token of another record", "/permits/43/supplement", ok, withCookie(cookie), 403, "form_token_mismatch"},
		{"cross-site origin", "/permits/42/supplement", ok, http.Header{"Cookie": {bindingCookie + "=" + cookie}, "Origin": {"https://evil.example"}}, 403, "cross_site_request"},
		{"cross-site fetch", "/permits/42/supplement", ok, http.Header{"Cookie": {bindingCookie + "=" + cookie}, "Sec-Fetch-Site": {"cross-site"}}, 403, "cross_site_request"},
		{"html in text", "/permits/42/supplement", url.Values{"message": {"<script>x</script>"}, "_eq_token": {token}}, withCookie(cookie), 400, "rule_violation"},
		{"character outside the legacy charset", "/permits/42/supplement", url.Values{"message": {"price €5"}, "_eq_token": {token}}, withCookie(cookie), 400, "charset_violation"},
		{"missing field", "/permits/42/supplement", url.Values{"_eq_token": {token}}, withCookie(cookie), 400, "missing_required"},
	}
	for _, c := range cases {
		rec := post(h, c.target, c.form, c.header)
		if rec.Code != c.status {
			t.Errorf("%s: status %d, want %d", c.name, rec.Code, c.status)
			continue
		}
		if ev := lastEvent(t, &events); ev["reason"] != c.reason {
			t.Errorf("%s: reason %v, want %s", c.name, ev["reason"], c.reason)
		}
	}

	req := httptest.NewRequest("POST", "/permits/42/supplement", strings.NewReader("--x--"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("multipart: %d", rec.Code)
	}
	big := url.Values{"message": {strings.Repeat("a", MaxFormBody)}, "_eq_token": {token}}
	if rec := post(h, "/permits/42/supplement", big, withCookie(cookie)); rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("large body: %d", rec.Code)
	}
	if len(legacy.posts) != 0 {
		t.Fatalf("the legacy application received %d rejected submissions", len(legacy.posts))
	}
	// The same origin is allowed.
	hdr := withCookie(cookie)
	hdr.Set("Origin", "http://example.com")
	if rec := post(h, "/permits/42/supplement", ok, hdr); rec.Code != http.StatusSeeOther {
		t.Errorf("same origin: %d", rec.Code)
	}
}
