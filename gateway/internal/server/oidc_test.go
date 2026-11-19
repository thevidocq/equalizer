package server

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/thevidocq/equalizer/gateway/internal/clientip"
	"github.com/thevidocq/equalizer/gateway/internal/formtoken"
	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/oidc"
	"github.com/thevidocq/equalizer/gateway/internal/session"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
)

// fakeIdP is an identity provider: discovery, keys, token endpoint. The
// test decides the groups of the next login and can break the signature.
type fakeIdP struct {
	srv     *httptest.Server
	key     *rsa.PrivateKey
	mu      sync.Mutex
	codes   map[string][2]string // code → nonce, PKCE challenge
	groups  []string
	tamper  bool
	secrets []string
}

func newIdP(t *testing.T) *fakeIdP {
	k, _ := rsa.GenerateKey(rand.Reader, 2048)
	f := &fakeIdP{key: k, codes: map[string][2]string{}}
	f.srv = httptest.NewTLSServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func enc(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func (f *fakeIdP) serve(w http.ResponseWriter, r *http.Request) {
	iss := f.srv.URL
	switch r.URL.Path {
	case "/.well-known/openid-configuration":
		_ = json.NewEncoder(w).Encode(map[string]string{"issuer": iss, "authorization_endpoint": iss + "/authorize",
			"token_endpoint": iss + "/token", "jwks_uri": iss + "/keys", "end_session_endpoint": iss + "/logout"})
	case "/keys":
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{"kty": "RSA", "kid": "k1", "use": "sig",
			"n": enc(f.key.N.Bytes()), "e": enc(big.NewInt(int64(f.key.E)).Bytes())}}})
	case "/token":
		_ = r.ParseForm()
		_, secret, _ := r.BasicAuth()
		f.mu.Lock()
		f.secrets = append(f.secrets, secret)
		c, ok := f.codes[r.Form.Get("code")]
		f.mu.Unlock()
		if !ok || oidc.Challenge(r.Form.Get("code_verifier")) != c[1] {
			w.WriteHeader(400)
			_, _ = io.WriteString(w, `{"error":"invalid_grant"}`)
			return
		}
		claims := map[string]any{"iss": iss, "aud": "eq-portal", "sub": "u-42", "email": "ann@acme.example",
			"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(), "nonce": c[0], "groups": f.groups}
		_ = json.NewEncoder(w).Encode(map[string]string{"id_token": f.sign(claims)})
	}
}

func (f *fakeIdP) sign(claims map[string]any) string {
	h, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "k1", "typ": "JWT"})
	p, _ := json.Marshal(claims)
	signed := enc(h) + "." + enc(p)
	sum := sha256.Sum256([]byte(signed))
	sig, _ := rsa.SignPKCS1v15(rand.Reader, f.key, crypto.SHA256, sum[:])
	if f.tamper {
		sig[0] ^= 1
	}
	return signed + "." + enc(sig)
}

// authorize plays the provider's login page: it returns the callback URL.
func (f *fakeIdP) authorize(t *testing.T, loc string) string {
	u, err := url.Parse(loc)
	if err != nil || !strings.HasPrefix(loc, f.srv.URL+"/authorize") {
		t.Fatalf("not sent to the provider: %q", loc)
	}
	q := u.Query()
	if q.Get("code_challenge_method") != "S256" || q.Get("redirect_uri") != "https://portal.acme.example/.eq/oidc/callback" {
		t.Fatalf("authorize request: %v", q)
	}
	code := oidc.Random()
	f.mu.Lock()
	f.codes[code] = [2]string{q.Get("nonce"), q.Get("code_challenge")}
	f.mu.Unlock()
	return "/.eq/oidc/callback?" + url.Values{"code": {code}, "state": {q.Get("state")}}.Encode()
}

type oneSecret string

func (s oneSecret) OIDCSecret(string) string { return string(s) }

func TestOIDC(t *testing.T) {
	idp := newIdP(t)
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, "<p>reports</p>")
	}))
	defer legacy.Close()
	u, _ := url.Parse(legacy.URL)
	m := &mapmodel.Model{
		Meta:   mapmodel.Meta{Tenant: "t", App: "portal", Version: 1, Status: "published", PublicDomain: "portal.acme.example"},
		Legacy: mapmodel.Legacy{BaseURL: legacy.URL, Reach: mapmodel.Reach{Mode: "direct", Allow: []string{u.Host}}},
		Roles: mapmodel.Roles{List: map[string]any{"staff": nil, "boss": nil},
			Auth: &mapmodel.Auth{Provider: "oidc", Issuer: idp.srv.URL, ClientID: "eq-portal",
				Roles: []mapmodel.AuthRole{{Group: "managers", Role: "boss"}, {Group: "employees", Role: "staff"}}}},
		Routes: []mapmodel.Route{
			{ID: "reports", Path: "/reports", Method: "GET", Roles: []string{"boss"}, LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/r"}},
			{ID: "home", Path: "/", Method: "GET", Roles: []string{"staff", "boss"}, LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/"}},
		},
	}
	st, err := site.Build(m)
	if err != nil {
		t.Fatal(err)
	}
	st.OIDC.Client = idp.srv.Client()
	sealer, _ := formtoken.New(formtoken.RandomKey())
	mgr, _ := session.NewManager(session.NewMemory(), formtoken.RandomKey())
	p := &Public{Maps: fixedMaps{st}, IPs: clientip.New(nil), Forms: sealer, Sessions: mgr, OIDCSecrets: oneSecret("s3cret"),
		Log: slog.New(slog.NewJSONHandler(io.Discard, nil)), Events: telemetry.NewEmitter(io.Discard, "test")}
	login := func(groups ...string) (*browser, *httptest.ResponseRecorder) {
		idp.groups = groups
		b := &browser{t: t, h: p.Handler(), cookies: map[string]string{}}
		rec := b.do("GET", "/reports?y=2026", nil)
		if rec.Code != http.StatusFound {
			t.Fatalf("no login redirect: %d", rec.Code)
		}
		return b, b.do("GET", idp.authorize(t, rec.Header().Get("Location")), nil)
	}

	b, rec := login("managers")
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/reports?y=2026" {
		t.Fatalf("callback: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec := b.do("GET", "/reports", nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), "reports") {
		t.Fatalf("after login: %d", rec.Code)
	}
	if idp.secrets[len(idp.secrets)-1] != "s3cret" {
		t.Error("client secret not sent")
	}

	b, _ = login("employees")
	if rec := b.do("GET", "/reports", nil); rec.Code != http.StatusForbidden {
		t.Errorf("staff on a boss page: %d", rec.Code)
	}
	if rec := b.do("GET", "/.eq/oidc/logout", nil); rec.Code != http.StatusFound || !strings.HasPrefix(rec.Header().Get("Location"), idp.srv.URL+"/logout") {
		t.Errorf("logout: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec := b.do("GET", "/", nil); rec.Code != http.StatusFound {
		t.Errorf("after logout: %d", rec.Code)
	}

	if _, rec := login("visitors"); rec.Code != http.StatusForbidden {
		t.Errorf("no mapped group: %d", rec.Code)
	}
	idp.tamper = true
	if _, rec := login("managers"); rec.Code != http.StatusForbidden {
		t.Errorf("bad signature: %d", rec.Code)
	}
	idp.tamper = false

	// A callback that does not belong to this browser, a forged state.
	other := &browser{t: t, h: p.Handler(), cookies: map[string]string{}}
	if rec := other.do("GET", "/.eq/oidc/callback?code=x&state=y", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("callback without a login: %d", rec.Code)
	}
	b2 := &browser{t: t, h: p.Handler(), cookies: map[string]string{}}
	loc := b2.do("GET", "/", nil).Header().Get("Location")
	cb := idp.authorize(t, loc)
	if rec := b2.do("GET", strings.Replace(cb, "state=", "state=x", 1), nil); rec.Code != http.StatusBadRequest {
		t.Errorf("forged state: %d", rec.Code)
	}
	if rec := b2.do("GET", cb, nil); rec.Code != http.StatusBadRequest {
		t.Errorf("a refused callback leaves no login to finish: %d", rec.Code)
	}

	// Two tabs start a login: both callbacks still work.
	tabs := &browser{t: t, h: p.Handler(), cookies: map[string]string{}}
	idp.groups = []string{"employees"}
	first := idp.authorize(t, tabs.do("GET", "/", nil).Header().Get("Location"))
	second := idp.authorize(t, tabs.do("GET", "/reports", nil).Header().Get("Location"))
	if rec := tabs.do("GET", second, nil); rec.Code != http.StatusFound {
		t.Fatalf("second tab: %d", rec.Code)
	}
	if rec := tabs.do("GET", first, nil); rec.Code != http.StatusFound {
		t.Errorf("first tab after the second: %d", rec.Code)
	}

	// A script's fetch (Sec-Fetch-Mode: cors) gets 401 even without asking for JSON.
	fr := httptest.NewRequest("GET", "/", nil)
	fr.Header.Set("Sec-Fetch-Mode", "cors")
	fw := httptest.NewRecorder()
	p.Handler().ServeHTTP(fw, fr)
	if fw.Code != http.StatusUnauthorized {
		t.Errorf("fetch without login: %d", fw.Code)
	}

	// APIs and writes get 401, not a redirect; the return path stays local.
	req := httptest.NewRequest("GET", "/reports", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	p.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("JSON without login: %d", rr.Code)
	}
	for in, want := range map[string]string{"//evil.example/x": "/", "/ok?a=1": "/ok?a=1", "https://evil": "/", "/.eq/oidc/login": "/"} {
		if got := safeReturn(in); got != want {
			t.Errorf("safeReturn(%q) = %q", in, got)
		}
	}

	bad := *m
	bad.Roles.Auth = &mapmodel.Auth{Provider: "oidc", Issuer: "http://idp", ClientID: "x", Roles: []mapmodel.AuthRole{{Group: "*", Role: "staff"}}}
	if _, err := site.Build(&bad); err == nil {
		t.Error("an http issuer was accepted")
	}
	bad.Roles.Auth = &mapmodel.Auth{Provider: "oidc", Issuer: idp.srv.URL, ClientID: "x", Roles: []mapmodel.AuthRole{{Group: "*", Role: "nobody"}}}
	if _, err := site.Build(&bad); err == nil {
		t.Error("a role not in the map was accepted")
	}
	bad = *m
	bad.Meta.PublicScheme = "http"
	if _, err := site.Build(&bad); err == nil || !strings.Contains(err.Error(), "https") {
		t.Errorf("OIDC on a site served in http: %v", err)
	}
}
