package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/site"
)

func TestPublicScheme(t *testing.T) {
	var legacyURL string
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "PHPSESSID", Value: "abc", HttpOnly: true}) // private: kept in the session
		http.SetCookie(w, &http.Cookie{Name: "lang", Value: "it"})                       // public: to the browser
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"home":"`+legacyURL+`/"}`)
	}))
	defer legacy.Close()
	legacyURL = legacy.URL
	build := func(scheme string) *Public {
		p := areaGateway(t, legacy.URL, "GET", "POST")
		st := p.Maps.(fixedMaps).s
		st.Model.Meta.PublicScheme = scheme
		rebuilt, err := site.Build(st.Model)
		if err != nil {
			t.Fatal(err)
		}
		p.Maps = fixedMaps{rebuilt}
		return p
	}
	get := func(h http.Handler, method string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/page?x=1", nil)
		req.Host = "blog.example"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	// An https site on the plain port: sent to https, method kept for POST.
	https := build("https")
	if rec := get(https.HandlerPlain(), "GET"); rec.Code != 301 || rec.Header().Get("Location") != "https://blog.example/page?x=1" {
		t.Errorf("https site in http: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if rec := get(https.HandlerPlain(), "POST"); rec.Code != 308 {
		t.Errorf("POST: %d", rec.Code)
	}
	rec := get(https.Handler(), "GET")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"https://blog.example/"`) ||
		!strings.Contains(rec.Header().Values("Set-Cookie")[0]+rec.Header().Values("Set-Cookie")[1], "__Host-eq_session") {
		t.Errorf("https site: %d %s %v", rec.Code, rec.Body.String(), rec.Header().Values("Set-Cookie"))
	}

	// An http site: served in clear, its own address in http, cookies the
	// browser keeps over http (no __Host-, no Secure).
	plain := build("http")
	rec = get(plain.HandlerPlain(), "GET")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"http://blog.example/"`) {
		t.Fatalf("http site: %d %s", rec.Code, rec.Body.String())
	}
	cookies := strings.Join(rec.Header().Values("Set-Cookie"), "\n")
	if !strings.Contains(cookies, "eq_session=") || strings.Contains(cookies, "__Host-") || strings.Contains(cookies, "Secure") ||
		!strings.Contains(cookies, "lang=it") || strings.Contains(cookies, "PHPSESSID") {
		t.Errorf("http site cookies:\n%s", cookies)
	}
}
