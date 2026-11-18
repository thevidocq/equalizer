package server

import (
	"bytes"
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

// loginLegacy is a legacy app with its own session: JSESSIONID is set on the
// login page, the login POST must carry it, and the restricted page needs
// the "auth" cookie.
type loginLegacy struct {
	mu       sync.Mutex
	sessions map[string]bool // JSESSIONID → logged in
	seen     []string        // Cookie headers received
}

func (l *loginLegacy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.seen = append(l.seen, r.Header.Get("Cookie"))
	w.Header().Set("Content-Type", "text/html")
	jsid, _ := r.Cookie("JSESSIONID")
	switch r.URL.Path {
	case "/login.jsp":
		if jsid == nil {
			http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: "legacy-sess-1", Path: "/"})
		}
		_, _ = io.WriteString(w, `<form method="post" action="login.do"><input name="j_username"><input type="password" name="j_password"></form>`)
	case "/login.do":
		_ = r.ParseForm()
		if jsid == nil || r.PostForm.Get("j_password") != "s3cret'pw" {
			_, _ = io.WriteString(w, "<p>Invalid credentials</p>")
			return
		}
		l.sessions[jsid.Value] = true
		http.SetCookie(w, &http.Cookie{Name: "auth", Value: "ok"})
		_, _ = io.WriteString(w, "<p>Welcome</p>")
	case "/area.jsp":
		if jsid == nil || !l.sessions[jsid.Value] {
			http.Redirect(w, r, "/login.jsp?expired=1", http.StatusFound)
			return
		}
		_, _ = io.WriteString(w, "<p>private area</p>")
	case "/logout.do":
		if jsid != nil {
			delete(l.sessions, jsid.Value)
		}
		http.SetCookie(w, &http.Cookie{Name: "auth", Value: "", MaxAge: -1})
		_, _ = io.WriteString(w, "<p>bye</p>")
	}
}

func sessionSite(t *testing.T, legacy http.Handler) *site.Site {
	t.Helper()
	srv := httptest.NewServer(legacy)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	m := &mapmodel.Model{
		Meta: mapmodel.Meta{Tenant: "t", App: "a", Version: 4},
		Legacy: mapmodel.Legacy{BaseURL: srv.URL, Reach: mapmodel.Reach{Mode: "direct", Allow: []string{u.Host}},
			SessionExpired: &mapmodel.SessionExpired{RedirectTo: "/login.jsp"}},
		Roles: mapmodel.Roles{Anonymous: "guest", List: map[string]any{"guest": nil, "citizen": nil}},
		Routes: []mapmodel.Route{
			{ID: "login_page", Path: "/login", Roles: []string{"guest", "citizen"},
				LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/login.jsp"}},
			{ID: "area", Path: "/area", Roles: []string{"citizen"},
				LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/area.jsp"}},
			{ID: "logout", Path: "/logout", Roles: []string{"citizen"}, Session: &mapmodel.SessionAction{End: true},
				LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/logout.do"}},
		},
		Forms: []mapmodel.Form{{
			ID: "login", Route: "/login", Roles: []string{"guest", "citizen"},
			Fields: map[string]mapmodel.FieldSpec{
				"username": {Type: "string", MaxLength: 32, Charset: "[a-z.]"},
				"password": {Type: "password", Rules: []string{"no_template_meta"}},
			},
			LegacyRequest: mapmodel.LegacyRequest{Method: "POST", Path: "/login.do",
				Form: map[string]string{"j_username": "{{fields.username}}", "j_password": "{{fields.password}}"}},
			Success:   &mapmodel.SuccessCheck{Cookie: "auth", NotContains: "Invalid credentials"},
			Session:   &mapmodel.SessionAction{Start: "citizen"},
			OnSuccess: &mapmodel.OnSuccess{Redirect: "area"},
		}},
	}
	st, err := site.Build(m)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

// browser keeps cookies like a real browser would (by name, for this host).
type browser struct {
	t       *testing.T
	h       http.Handler
	cookies map[string]string
}

func (b *browser) do(method, target string, form url.Values) *httptest.ResponseRecorder {
	b.t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req := httptest.NewRequest(method, target, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for k, v := range b.cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	rec := httptest.NewRecorder()
	b.h.ServeHTTP(rec, req)
	for _, c := range rec.Result().Cookies() {
		if c.MaxAge < 0 {
			delete(b.cookies, c.Name)
		} else {
			b.cookies[c.Name] = c.Value
		}
	}
	return rec
}

func TestLoginSessionLogout(t *testing.T) {
	legacy := &loginLegacy{sessions: map[string]bool{}}
	st := sessionSite(t, legacy)
	sealer, _ := formtoken.New(formtoken.RandomKey())
	mgr, _ := session.NewManager(session.NewMemory(), formtoken.RandomKey())
	var events bytes.Buffer
	p := &Public{Maps: fixedMaps{st}, IPs: clientip.New(nil), Forms: sealer, Sessions: mgr,
		Log: slog.New(slog.NewJSONHandler(io.Discard, nil)), Events: telemetry.NewEmitter(&events, "test")}
	b := &browser{t: t, h: p.Handler(), cookies: map[string]string{}}

	if rec := b.do("GET", "/area", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("anonymous on a citizen page: %d", rec.Code)
	}
	rec := b.do("GET", "/login", nil)
	if rec.Code != 200 || strings.Contains(rec.Header().Get("Set-Cookie"), "JSESSIONID") {
		t.Fatalf("login page: %d, legacy cookie leaked: %v", rec.Code, rec.Header()["Set-Cookie"])
	}
	anonID := b.cookies[session.CookieName]
	if anonID == "" || rec.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("pre-login session not created: %v", rec.Header())
	}
	token := tokenRe.FindStringSubmatch(rec.Body.String())[1]

	// Wrong password: no role, legacy page shown.
	rec = b.do("POST", "/login", url.Values{"username": {"mary"}, "password": {"nope"}, "_eq_token": {token}})
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Invalid credentials") {
		t.Fatalf("failed login: %d %s", rec.Code, rec.Body.String())
	}
	if rec := b.do("GET", "/area", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("role granted after a failed login: %d", rec.Code)
	}

	rec = b.do("POST", "/login", url.Values{"username": {"mary"}, "password": {"s3cret'pw"}, "_eq_token": {token}})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/area" {
		t.Fatalf("login: %d %q %s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	if b.cookies[session.CookieName] == anonID {
		t.Fatal("the session id must change at login")
	}
	rec = b.do("GET", "/area", nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "private area") {
		t.Fatalf("area after login: %d %s", rec.Code, rec.Body.String())
	}
	if last := legacy.seen[len(legacy.seen)-1]; !strings.Contains(last, "JSESSIONID=legacy-sess-1") || !strings.Contains(last, "auth=ok") {
		t.Errorf("legacy cookies not sent: %q", last)
	}
	// The old (pre-login) id no longer works.
	stolen := &browser{t: t, h: b.h, cookies: map[string]string{session.CookieName: anonID}}
	if rec := stolen.do("GET", "/area", nil); rec.Code != http.StatusForbidden {
		t.Errorf("pre-login id still valid: %d", rec.Code)
	}
	if strings.Contains(events.String(), b.cookies[session.CookieName]) || strings.Contains(events.String(), "s3cret") {
		t.Fatal("session id or password in the events")
	}

	// The legacy session ends on its side: the gateway drops the role.
	legacy.mu.Lock()
	legacy.sessions = map[string]bool{}
	legacy.mu.Unlock()
	rec = b.do("GET", "/area", nil)
	// /login.jsp?expired=1 has an extra query key, so it is not the login
	// route: a real map declares it in compat. Here it stays a relative path.
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/login.jsp?expired=1" {
		t.Fatalf("expired legacy session: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec := b.do("GET", "/area", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("role kept after legacy expiry: %d", rec.Code)
	}
}

func TestLogoutEndsSession(t *testing.T) {
	legacy := &loginLegacy{sessions: map[string]bool{"legacy-sess-1": true}}
	st := sessionSite(t, legacy)
	store := session.NewMemory()
	mgr, _ := session.NewManager(store, formtoken.RandomKey())
	p := &Public{Maps: fixedMaps{st}, IPs: clientip.New(nil), Sessions: mgr,
		Log: slog.New(slog.NewJSONHandler(io.Discard, nil)), Events: telemetry.NewEmitter(io.Discard, "test")}
	id := session.NewID()
	_ = mgr.Save(t.Context(), id, &session.Session{Tenant: "t", App: "a", Role: "citizen",
		Cookies: map[string]string{"JSESSIONID": "legacy-sess-1", "auth": "ok"}}, st.SessionPolicy())
	b := &browser{t: t, h: p.Handler(), cookies: map[string]string{session.CookieName: id}}

	if rec := b.do("GET", "/area", nil); rec.Code != 200 {
		t.Fatalf("area: %d", rec.Code)
	}
	if rec := b.do("GET", "/logout", nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), "bye") {
		t.Fatalf("logout: %d", rec.Code)
	}
	if _, ok := b.cookies[session.CookieName]; ok {
		t.Error("session cookie not cleared")
	}
	if _, err := mgr.Load(t.Context(), id, "t", "a", st.SessionPolicy()); err == nil {
		t.Error("session still in the store after logout")
	}
}

func TestNoSessionStoreRefusesLogin(t *testing.T) {
	st := sessionSite(t, &loginLegacy{sessions: map[string]bool{}})
	sealer, _ := formtoken.New(formtoken.RandomKey())
	p := &Public{Maps: fixedMaps{st}, IPs: clientip.New(nil), Forms: sealer,
		Log: slog.New(slog.NewJSONHandler(io.Discard, nil)), Events: telemetry.NewEmitter(io.Discard, "test")}
	b := &browser{t: t, h: p.Handler(), cookies: map[string]string{}}
	if rec := b.do("GET", "/login", nil); rec.Code != 200 || b.cookies[session.CookieName] != "" {
		t.Fatalf("login page without sessions: %d %v", rec.Code, b.cookies)
	}
	if rec := b.do("POST", "/login", url.Values{"username": {"m"}, "password": {"x"}}); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("login without a session store: %d", rec.Code)
	}
}
