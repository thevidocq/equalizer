package server

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/thevidocq/equalizer/gateway/internal/formtoken"
	"github.com/thevidocq/equalizer/gateway/internal/mapstore"
	"github.com/thevidocq/equalizer/gateway/internal/session"
)

const tryToken = "service-token"

func tryMapDoc(legacy string) string {
	addr := strings.TrimPrefix(legacy, "http://")
	return `{"meta":{"schema":"gateway-map/1.0","tenant":"acme","app":"blog","version":4,"status":"draft",
	  "public_domain":"blog.acme.example"},
	 "legacy":{"base_url":"` + legacy + `","reach":{"mode":"direct","allow":["` + addr + `"]}},
	 "roles":{"anonymous":"guest","list":{"guest":{},"editor":{}}},
	 "routes":[{"id":"home","path":"/","method":"GET","roles":["guest","editor"],
	   "legacy_request":{"method":"GET","path":"/","query":{"p":"{{query.p}}"}},
	   "query":{"p":{"type":"int","min":1,"max":1000,"required":false}}}]}`
}

func signTry(t *testing.T, key ed25519.PrivateKey, doc string, purpose string, expires time.Time) mapstore.TryMap {
	t.Helper()
	payload := []byte(doc)
	return mapstore.TryMap{Payload: base64.StdEncoding.EncodeToString(payload),
		Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(key, mapstore.TryMessage(expires, payload))),
		KeyID:     "k1", Purpose: purpose, ExpiresAt: expires.UTC().Truncate(time.Second)}
}

func TestTry(t *testing.T) {
	var hits atomic.Int32
	var lastCookie atomic.Value
	lastCookie.Store("")
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		lastCookie.Store(r.Header.Get("Cookie"))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, `<html><body><a href="`+"http://"+r.Host+`/?p=2">next</a> page `+r.URL.Query().Get("p")+`</body></html>`)
	}))
	defer legacy.Close()

	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	sealer, _ := formtoken.New(formtoken.RandomKey())
	store := session.NewMemory()
	sk := formtoken.RandomKey()
	mgr, _ := session.NewManager(store, sk)
	frozen, _ := session.NewManager(session.ReadOnly(store), sk)
	now := time.Now()
	tr := &Try{Token: tryToken, Pub: pub, KeyID: "k1", Sessions: mgr, Frozen: frozen, Forms: sealer,
		Log: slog.New(slog.NewJSONHandler(io.Discard, nil)), now: func() time.Time { return now }}
	h := (&Admin{Try: tr}).Handler()
	doc := tryMapDoc(legacy.URL)
	good := signTry(t, key, doc, "try", now.Add(10*time.Minute))

	call := func(token string, body any) (int, tryResult) {
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/maps/try", strings.NewReader(string(b)))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var res tryResult
		_ = json.Unmarshal(rec.Body.Bytes(), &res)
		return rec.Code, res
	}
	try := func(m mapstore.TryMap, mode, method, path string, sess any) map[string]any {
		return map[string]any{"map": m, "mode": mode, "session": sess,
			"request": map[string]any{"method": method, "path": path}}
	}

	t.Run("refused callers and maps", func(t *testing.T) {
		if c, _ := call("", try(good, "dry_run", "GET", "/", nil)); c != 401 {
			t.Errorf("no token: %d", c)
		}
		for name, m := range map[string]mapstore.TryMap{
			"other purpose": signTry(t, key, doc, "publish", now.Add(time.Minute)),
			"expired":       signTry(t, key, doc, "try", now.Add(-time.Second)),
			"too far ahead": signTry(t, key, doc, "try", now.Add(3*time.Hour)),
			"other key": func() mapstore.TryMap {
				_, k2, _ := ed25519.GenerateKey(rand.Reader)
				return signTry(t, k2, doc, "try", now.Add(time.Minute))
			}(),
			"changed map": func() mapstore.TryMap {
				m := good
				m.Payload = base64.StdEncoding.EncodeToString([]byte(strings.Replace(doc, `"max":1000`, `"max":9999`, 1)))
				return m
			}(),
		} {
			if c, _ := call(tryToken, try(m, "dry_run", "GET", "/", nil)); c != 403 {
				t.Errorf("%s: %d", name, c)
			}
		}
		if c, _ := call(tryToken, try(good, "preview", "POST", "/", nil)); c != 403 {
			t.Errorf("preview POST: %d", c)
		}
	})

	t.Run("dry run never reaches the legacy site", func(t *testing.T) {
		before := hits.Load()
		c, res := call(tryToken, try(good, "dry_run", "GET", "/?p=7", nil))
		if c != 200 || !res.Accepted || res.Entry == nil || *res.Entry != "home" || res.Area {
			t.Fatalf("valid value: %d %+v", c, res)
		}
		for _, attack := range []string{"/?p=1%20OR%201=1", "/?p=%27--", "/?p=99999", "/?p=%3Cscript%3E"} {
			c, res := call(tryToken, try(good, "dry_run", "GET", attack, nil))
			if c != 200 || res.Accepted || res.Refusal == nil {
				t.Errorf("%s accepted: %+v", attack, res)
			}
		}
		c, res = call(tryToken, try(good, "dry_run", "GET", "/wp-login.php", nil))
		if res.Accepted || res.Refusal == nil || res.Refusal.Status != 404 {
			t.Errorf("unmapped path: %d %+v", c, res)
		}
		if hits.Load() != before {
			t.Errorf("dry run reached the legacy site %d times", hits.Load()-before)
		}
	})

	t.Run("preview serves the rebuilt page", func(t *testing.T) {
		c, res := call(tryToken, try(good, "preview", "GET", "/?p=3", nil))
		if c != 200 || !res.Accepted || res.Response == nil || res.Response.Status != 200 {
			t.Fatalf("preview: %d %+v", c, res)
		}
		body, _ := base64.StdEncoding.DecodeString(res.Response.Body)
		if !strings.Contains(string(body), "page 3") || !strings.Contains(string(body), `href="/?p=2"`) {
			t.Errorf("not the rebuilt page: %s", body)
		}
	})

	t.Run("a test account's session", func(t *testing.T) {
		if c, _ := call(tryToken, try(good, "preview", "GET", "/", map[string]any{"role": "admin"})); c != 400 {
			t.Errorf("unknown role: %d", c)
		}
		c, res := call(tryToken, try(good, "preview", "GET", "/",
			map[string]any{"role": "editor", "legacy_cookies": map[string]string{"wp_logged_in": "abc"}}))
		if c != 200 || !res.Accepted || len(res.Cookies) == 0 {
			t.Fatalf("seeded: %d %+v", c, res)
		}
		if got := lastCookie.Load().(string); got != "wp_logged_in=abc" {
			t.Errorf("legacy cookie sent: %q", got)
		}
	})
}

func TestTryOff(t *testing.T) {
	for _, a := range []*Admin{{}, {Try: &Try{}}} {
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/maps/try", strings.NewReader("{}")))
		if rec.Code != 404 {
			t.Errorf("tries off: %d", rec.Code)
		}
	}
}

// An http site's forms: the token is checked against the plain binding
// cookie (found by the agent's verification of the lab intranet).
func TestTryFormOnHTTPSite(t *testing.T) {
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, `<html><body><form method="post" action="/login"><input name="user"><button>Go</button></form></body></html>`)
	}))
	defer legacy.Close()
	addr := strings.TrimPrefix(legacy.URL, "http://")
	doc := `{"meta":{"schema":"gateway-map/1.0","tenant":"acme","app":"intranet","version":2,"status":"draft",
	  "public_domain":"intranet.acme.example","public_scheme":"http"},
	 "legacy":{"base_url":"` + legacy.URL + `","reach":{"mode":"direct","allow":["` + addr + `"]}},
	 "roles":{"anonymous":"guest","list":{"guest":{}}},
	 "routes":[{"id":"login_page","path":"/login","method":"GET","roles":["guest"],"legacy_request":{"method":"GET","path":"/login"}}],
	 "forms":[{"id":"login","route":"/login","roles":["guest"],"fields":{"user":{"type":"string","max_length":20}},
	   "legacy_request":{"method":"POST","path":"/login","form":{"user":"{{fields.user}}"}}}]}`
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	sealer, _ := formtoken.New(formtoken.RandomKey())
	store := session.NewMemory()
	sk := formtoken.RandomKey()
	mgr, _ := session.NewManager(store, sk)
	frozen, _ := session.NewManager(session.ReadOnly(store), sk)
	now := time.Now()
	h := (&Admin{Try: &Try{Token: tryToken, Pub: pub, KeyID: "k1", Sessions: mgr, Frozen: frozen, Forms: sealer,
		Log: slog.New(slog.NewJSONHandler(io.Discard, nil)), now: func() time.Time { return now }}}).Handler()
	m := signTry(t, key, doc, "try", now.Add(10*time.Minute))
	call := func(body map[string]any) tryResult {
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/maps/try", strings.NewReader(string(b)))
		req.Header.Set("Authorization", "Bearer "+tryToken)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var res tryResult
		_ = json.Unmarshal(rec.Body.Bytes(), &res)
		if rec.Code != 200 {
			t.Fatalf("try: %d %s", rec.Code, rec.Body.String())
		}
		return res
	}
	page := call(map[string]any{"map": m, "mode": "preview", "request": map[string]any{"method": "GET", "path": "/login"}})
	body, _ := base64.StdEncoding.DecodeString(page.Response.Body)
	tok := regexp.MustCompile(`name="_eq_token" value="([^"]+)"`).FindSubmatch(body)
	if tok == nil {
		t.Fatalf("no form token in %s", body)
	}
	var cookies []string
	for _, c := range page.Cookies {
		cookies = append(cookies, strings.SplitN(c, ";", 2)[0])
	}
	post := call(map[string]any{"map": m, "mode": "dry_run", "request": map[string]any{"method": "POST", "path": "/login",
		"headers": map[string]string{"Cookie": strings.Join(cookies, "; "), "Content-Type": "application/x-www-form-urlencoded"},
		"body":    base64.StdEncoding.EncodeToString([]byte("user=bob&_eq_token=" + url.QueryEscape(string(tok[1]))))}})
	if !post.Accepted {
		t.Fatalf("a valid submission on an http site is refused: %+v (cookies %v)", post.Refusal, cookies)
	}
}
