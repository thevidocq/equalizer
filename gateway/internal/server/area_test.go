package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/clientip"
	"github.com/thevidocq/equalizer/gateway/internal/formtoken"
	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/session"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
)

type seenReq struct {
	method, uri, body, ctype, cookie, referer, xff string
}

func TestAreas(t *testing.T) {
	var mu sync.Mutex
	var seen []seenReq
	var legacyURL string
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen = append(seen, seenReq{r.Method, r.URL.RequestURI(), string(b), r.Header.Get("Content-Type"), r.Header.Get("Cookie"),
			r.Header.Get("Referer"), r.Header.Get("X-Forwarded-For")})
		mu.Unlock()
		switch r.URL.Path {
		case "/home":
			_, _ = io.WriteString(w, "<html><body>home</body></html>")
		case "/wp-admin/edit.php":
			http.SetCookie(w, &http.Cookie{Name: "wordpress_logged_in", Value: "abc"})
			w.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(w, `<html><body><a href="`+legacyURL+`/wp-admin/post.php?post=1&action=edit">Edit</a>
<form method="post" action="/wp-admin/post.php"><input type="hidden" name="_wpnonce" value="n0nce"><input name="title"></form></body></html>`)
		case "/wp-admin/admin-ajax.php":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(400)
			_, _ = io.WriteString(w, `{"success":false}`)
		case "/wp-json/wp/v2/posts":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(201)
			_, _ = io.WriteString(w, `{"id":7}`)
		case "/backup.zip":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write([]byte("PK\x03\x04binary"))
		case "/go-out":
			http.Redirect(w, r, "https://payments.example/pay", http.StatusFound)
		case "/go-in":
			http.Redirect(w, r, legacyURL+"/wp-admin/edit.php", http.StatusFound)
		default:
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(403)
			_, _ = io.WriteString(w, "<html><body>Sorry, you are not allowed</body></html>")
		}
	}))
	defer legacy.Close()
	legacyURL = legacy.URL
	u, _ := url.Parse(legacy.URL)
	m := &mapmodel.Model{
		Meta:   mapmodel.Meta{Tenant: "t", App: "blog", Version: 1, Status: "published", PublicDomain: "blog.example"},
		Legacy: mapmodel.Legacy{BaseURL: legacy.URL, Reach: mapmodel.Reach{Mode: "direct", Allow: []string{u.Host}}},
		Roles:  mapmodel.Roles{Anonymous: "guest", List: map[string]any{"guest": nil}},
		Routes: []mapmodel.Route{{ID: "home", Path: "/", Method: "GET", Roles: []string{"guest"},
			LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/home"}}},
		Areas: []mapmodel.Area{{ID: "site", Prefix: "/", Methods: []string{"GET", "POST", "PUT"}}},
	}
	st, err := site.Build(m)
	if err != nil {
		t.Fatal(err)
	}
	sealer, _ := formtoken.New(formtoken.RandomKey())
	mgr, _ := session.NewManager(session.NewMemory(), formtoken.RandomKey())
	p := &Public{Maps: fixedMaps{st}, IPs: clientip.New(nil), Forms: sealer, Sessions: mgr,
		Log: slog.New(slog.NewJSONHandler(io.Discard, nil)), Events: telemetry.NewEmitter(io.Discard, "test")}
	jar := map[string]string{}
	do := func(method, target, ctype, body string, hdr ...string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, target, strings.NewReader(body))
		req.Host = "blog.example"
		if ctype != "" {
			req.Header.Set("Content-Type", ctype)
		}
		for i := 0; i+1 < len(hdr); i += 2 {
			req.Header.Set(hdr[i], hdr[i+1])
		}
		for k, v := range jar {
			req.AddCookie(&http.Cookie{Name: k, Value: v})
		}
		rec := httptest.NewRecorder()
		p.Handler().ServeHTTP(rec, req)
		for _, c := range rec.Result().Cookies() {
			jar[c.Name] = c.Value
		}
		return rec
	}
	last := func() seenReq { mu.Lock(); defer mu.Unlock(); return seen[len(seen)-1] }

	// A precise entry wins over the area.
	if rec := do("GET", "/", "", ""); rec.Code != 200 || last().uri != "/home" {
		t.Fatalf("home: %d %s", rec.Code, last().uri)
	}
	// Admin pages are served like the rest; links and hidden values work.
	rec := do("GET", "/wp-admin/edit.php?post_type=page&paged=2", "", "", "Referer", "https://blog.example/wp-admin/", "X-Forwarded-For", "1.2.3.4", "Cookie", "evil=1")
	body := rec.Body.String()
	if rec.Code != 200 || last().uri != "/wp-admin/edit.php?post_type=page&paged=2" {
		t.Fatalf("admin page: %d %s", rec.Code, last().uri)
	}
	if strings.Contains(body, legacy.URL) || !strings.Contains(body, `href="/wp-admin/post.php?post=1&amp;action=edit"`) ||
		!strings.Contains(body, `name="_wpnonce" value="n0nce"`) {
		t.Errorf("rendered admin page:\n%s", body)
	}
	if got := last(); got.cookie != "" || got.xff != "" || got.referer != legacy.URL+"/wp-admin/" {
		t.Errorf("headers towards the legacy app: %+v", got)
	}
	if strings.Contains(rec.Header().Get("Set-Cookie"), "wordpress") {
		t.Error("a legacy cookie reached the browser")
	}

	// Forms of the admin: rebuilt, order kept, legacy status and body passed on.
	rec = do("POST", "/wp-admin/admin-ajax.php", "application/x-www-form-urlencoded", "action=heartbeat&data%5Bx%5D=1&data%5Bx%5D=2")
	if rec.Code != 400 || rec.Body.String() != `{"success":false}` || last().body != "action=heartbeat&data%5Bx%5D=1&data%5Bx%5D=2" {
		t.Errorf("ajax: %d %q %q", rec.Code, rec.Body.String(), last().body)
	}
	if !strings.Contains(last().cookie, "wordpress_logged_in=abc") {
		t.Errorf("legacy session not kept: %q", last().cookie)
	}
	rec = do("POST", "/wp-json/wp/v2/posts", "application/json", ` { "title" : "Hi" } `)
	if rec.Code != 201 || last().body != `{"title":"Hi"}` {
		t.Errorf("json: %d %q", rec.Code, last().body)
	}
	if rec := do("GET", "/backup.zip", "", ""); rec.Code != 200 || rec.Body.String() != "PK\x03\x04binary" {
		t.Errorf("download: %d", rec.Code)
	}
	if rec := do("GET", "/private", "", ""); rec.Code != 403 || !strings.Contains(rec.Body.String(), "not allowed") {
		t.Errorf("legacy 403 not passed on: %d", rec.Code)
	}
	if rec := do("GET", "/go-out", "", ""); rec.Code != 302 || rec.Header().Get("Location") != "https://payments.example/pay" {
		t.Errorf("external redirect: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec := do("GET", "/go-in", "", ""); rec.Header().Get("Location") != "/wp-admin/edit.php" {
		t.Errorf("internal redirect: %q", rec.Header().Get("Location"))
	}

	// What cannot be rebuilt never reaches the legacy application.
	n := len(seen)
	for name, c := range map[string]struct {
		method, target, ctype, body string
		code                        int
	}{
		"XXE":         {"POST", "/xmlrpc.php", "text/xml", `<!DOCTYPE r [<!ENTITY x SYSTEM "file:///etc/passwd">]><r>&x;</r>`, 400},
		"binary":      {"POST", "/upload", "application/octet-stream", "\x00\x01", 415},
		"bad query":   {"GET", "/search?q=%zz", "", "", 400},
		"NUL":         {"POST", "/form", "application/x-www-form-urlencoded", "a=%00", 400},
		"method":      {"DELETE", "/wp-json/wp/v2/posts/7", "", "", 405},
		"dot segment": {"GET", "/a/%2e%2e/etc", "", "", 400},
	} {
		if rec := do(c.method, c.target, c.ctype, c.body); rec.Code != c.code {
			t.Errorf("%s: %d, want %d", name, rec.Code, c.code)
		}
	}
	if len(seen) != n {
		t.Errorf("refused requests reached the legacy app: %v", seen[n:])
	}
}
