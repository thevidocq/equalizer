package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/thevidocq/equalizer/gateway/internal/clientip"
	"github.com/thevidocq/equalizer/gateway/internal/formtoken"
	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/session"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
)

func TestLegacyCookies(t *testing.T) {
	var got string
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Cookie")
		if r.URL.Path == "/login" {
			http.SetCookie(w, &http.Cookie{Name: "PHPSESSID", Value: "s1", HttpOnly: true})
			http.SetCookie(w, &http.Cookie{Name: "wordpress_logged_in", Value: "u1", Expires: time.Now().Add(14 * 24 * time.Hour)})
			http.SetCookie(w, &http.Cookie{Name: "pll_language", Value: "it", MaxAge: 3600})
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, "<html><body>ok</body></html>")
	}))
	defer legacy.Close()
	p := areaPublic(t, legacy.URL, 0)
	do := func(target string, cookies ...*http.Cookie) *http.Response {
		req := httptest.NewRequest("GET", target, nil)
		req.Host = "blog.example"
		for _, c := range cookies {
			req.AddCookie(c)
		}
		rec := httptest.NewRecorder()
		p.Handler().ServeHTTP(rec, req)
		return rec.Result()
	}

	res := do("/login")
	var gw, lang *http.Cookie
	for _, c := range res.Cookies() {
		switch c.Name {
		case session.CookieName:
			gw = c
		case "pll_language":
			lang = c
		case "PHPSESSID", "wordpress_logged_in":
			t.Errorf("private legacy cookie %s reached the browser", c.Name)
		}
	}
	if lang == nil || lang.Value != "it" || !lang.Secure {
		t.Errorf("public legacy cookie: %+v", lang)
	}
	// Remember me: the gateway session outlives the browser, like the legacy login.
	if gw == nil || gw.Expires.Before(time.Now().Add(13*24*time.Hour)) {
		t.Fatalf("gateway session cookie: %+v", gw)
	}

	// Next page: private cookies from the jar, public ones from the browser
	// (including one written by the page's scripts); forged private names dropped.
	do("/page", gw, lang, &http.Cookie{Name: "cookie_consent", Value: "yes"},
		&http.Cookie{Name: "PHPSESSID", Value: "forged"})
	for _, want := range []string{"PHPSESSID=s1", "wordpress_logged_in=u1", "pll_language=it", "cookie_consent=yes"} {
		if !strings.Contains(got, want) {
			t.Errorf("legacy request misses %s: %q", want, got)
		}
	}
	if strings.Contains(got, "forged") {
		t.Errorf("forged cookie reached the legacy app: %q", got)
	}
}

// areaPublic serves a legacy site through one area covering it all.
func areaPublic(t *testing.T, legacyURL string, maxKB int) *Public {
	t.Helper()
	u, _ := url.Parse(legacyURL)
	m := &mapmodel.Model{
		Meta:   mapmodel.Meta{Tenant: "t", App: "blog", Version: 1, Status: "published", PublicDomain: "blog.example"},
		Legacy: mapmodel.Legacy{BaseURL: legacyURL, Reach: mapmodel.Reach{Mode: "direct", Allow: []string{u.Host}}},
		Roles:  mapmodel.Roles{Anonymous: "guest", List: map[string]any{"guest": nil}},
		Areas:  []mapmodel.Area{{ID: "site", Prefix: "/", Methods: []string{"GET", "POST"}, MaxBodyKB: maxKB}},
	}
	st, err := site.Build(m)
	if err != nil {
		t.Fatal(err)
	}
	sealer, _ := formtoken.New(formtoken.RandomKey())
	mgr, _ := session.NewManager(session.NewMemory(), formtoken.RandomKey())
	return &Public{Maps: fixedMaps{st}, IPs: clientip.New(nil), Forms: sealer, Sessions: mgr,
		Log: slog.New(slog.NewJSONHandler(io.Discard, nil)), Events: telemetry.NewEmitter(io.Discard, "test")}
}

func TestAbsorbRules(t *testing.T) {
	now := time.Now()
	ss := &sessionState{policy: session.DefaultPolicy, keepAll: true}
	// A login answer: every cookie is private, whatever its name; a stale
	// cookie deleted in the same answer does not undo "remember me".
	ss.absorb([]*http.Cookie{
		{Name: "ck", Value: "login", Expires: now.Add(48 * time.Hour)},
		{Name: "old", MaxAge: -1},
	})
	if ss.s == nil || ss.s.Cookies["ck"] != "login" || len(ss.out) != 0 || !ss.s.Remembered(now.Add(24*time.Hour)) {
		t.Fatalf("login cookies: %+v out=%v", ss.s, ss.out)
	}
	// Logout on the legacy site: the login cookie goes, and so does "remember me".
	ss.keepAll = false
	ss.absorb([]*http.Cookie{{Name: "ck", MaxAge: -1}, {Name: "lang", Value: "it"}})
	if ss.s.Remembered(now) || ss.s.Cookies["ck"] != "" || len(ss.out) != 1 || ss.out[0].Name != "lang" {
		t.Errorf("after logout: %+v out=%v", ss.s, ss.out)
	}
	// A deletion alone never creates a session; public cookies still pass
	// when the store is down.
	fresh := &sessionState{policy: session.DefaultPolicy, broken: true}
	fresh.absorb([]*http.Cookie{{Name: "PHPSESSID", MaxAge: -1}, {Name: "theme", Value: "dark"}})
	if fresh.s != nil || len(fresh.out) != 1 {
		t.Errorf("broken store: %+v out=%v", fresh.s, fresh.out)
	}
}

func TestEventStreamThroughHandler(t *testing.T) {
	release := make(chan struct{})
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		<-release
	}))
	defer legacy.Close()
	p := areaPublic(t, legacy.URL, 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Host = "blog.example"
		p.Handler().ServeHTTP(w, r)
	}))
	defer srv.Close()
	defer close(release) // before the servers close, which wait for the handlers
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Get(srv.URL + "/live")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	got := make(chan string, 1)
	go func() {
		b := make([]byte, 64)
		n, _ := resp.Body.Read(b)
		got <- string(b[:n])
	}()
	select {
	case s := <-got:
		if s != "data: first\n\n" {
			t.Errorf("first event %q", s)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the first event was held back")
	}
}

func TestRequestAppHeader(t *testing.T) {
	for k, want := range map[string]bool{
		"X-WP-Nonce": true, "X-CSRF-Token": true, "X-Requested-With": true, "X-Auth-Token": true,
		"X-HTTP-Method-Override": false, "X-Method-Override": false, "X-Remote-User": false, "X-Auth-User": false,
		"X-User": false, "X-Middleware-Subrequest": false, "X-Original-URL": false, "X-Forwarded-For": false,
		"X-Real-IP": false, "X-MS-Client-Principal-Name": false, "X-Eq-Hop": false,
	} {
		if requestAppHeader(k) != want {
			t.Errorf("%s: want %v", k, want)
		}
	}
}
