package api_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/api"
	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/dbtest"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

var pg *dbtest.Server

func TestMain(m *testing.M) {
	var err error
	if pg, err = dbtest.Start(); err != nil {
		panic(err)
	}
	code := m.Run()
	pg.Stop()
	os.Exit(code)
}

const internalToken = "internal-token-0123456789abcdef0123456789"

type env struct {
	t   *testing.T
	srv *httptest.Server
	st  *store.Store
	pub ed25519.PublicKey
}

func setup(t *testing.T) *env {
	t.Helper()
	if pg == nil {
		t.Skip("PostgreSQL not installed")
	}
	conn := pg.NewDatabase(t)
	st := store.New(conn)
	pub, priv, _ := ed25519.GenerateKey(nil)
	s := &api.Server{
		Store: st, Sessions: &auth.Sessions{DB: conn, TTL: time.Hour}, Throttle: auth.NewThrottle(5, time.Minute),
		Log: slog.New(slog.NewJSONHandler(io.Discard, nil)), SigningKey: priv, KeyID: "k1", InternalToken: internalToken,
	}
	srv := httptest.NewTLSServer(s.Handler())
	t.Cleanup(srv.Close)
	// Bootstrap the platform administrator as `panel-api create-admin` does.
	hash, _ := auth.HashPassword("platform-admin-password")
	admin := "admin"
	if _, err := st.CreateUser(context.Background(), store.NewUser{Email: "root@example.com", Name: "Root", PasswordHash: hash, PlatformRole: &admin}); err != nil {
		t.Fatal(err)
	}
	return &env{t: t, srv: srv, st: st, pub: pub}
}

type client struct {
	e *env
	c *http.Client
}

func (e *env) client() *client {
	jar, _ := cookiejar.New(nil)
	// srv.Client() returns the same client every time: copy it, one jar each.
	c := *e.srv.Client()
	c.Jar = jar
	return &client{e: e, c: &c}
}

func (c *client) do(method, path string, body any, out any) int {
	c.e.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.e.srv.URL+path, rd)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Eq-Panel", "1")
	resp, err := c.c.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		_ = json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode
}

func (c *client) login(email, password string) int {
	return c.do("POST", "/v1/auth/login", map[string]string{"email": email, "password": password}, nil)
}

func (c *client) must(want int, method, path string, body any, out any) {
	c.e.t.Helper()
	if got := c.do(method, path, body, out); got != want {
		c.e.t.Fatalf("%s %s: status %d, want %d", method, path, got, want)
	}
}

const mapDoc = `{"meta":{"schema":"gateway-map/1.0","tenant":"x","app":"x","version":99,"status":"published"},
 "legacy":{"base_url":"http://10.0.0.1:8080","reach":{"mode":"connector","connector_id":"cn-1","allow":["10.0.0.1:8080"]}},
 "roles":{"anonymous":"guest","list":{"guest":{}}},
 "routes":[{"id":"home","path":"/","method":"GET","roles":["guest"],"legacy_request":{"method":"GET","path":"/"},
   "render":"faithful","origin":"human","risk":"low"}]}`

func TestPanelFlow(t *testing.T) {
	e := setup(t)
	root := e.client()
	if root.login("root@example.com", "wrong-password-123") != http.StatusUnauthorized {
		t.Fatal("wrong password accepted")
	}
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "ROOT@example.com", "password": "platform-admin-password"}, nil)
	var me auth.User
	root.must(200, "GET", "/v1/auth/me", nil, &me)
	if !me.PlatformAdmin() {
		t.Fatalf("me: %+v", me)
	}

	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "other", "name": "Other"}, nil)
	root.must(409, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Again"}, nil)
	for _, u := range []struct{ tenant, email, role string }{
		{"acme", "ed@acme.test", "editor"}, {"acme", "ap@acme.test", "approver"}, {"other", "boss@other.test", "admin"},
	} {
		root.must(201, "POST", "/v1/tenants/"+u.tenant+"/users", map[string]string{"email": u.email, "name": u.role, "password": "a-long-password-1", "role": u.role}, nil)
	}
	root.must(422, "POST", "/v1/tenants/acme/users", map[string]string{"email": "x@acme.test", "name": "x", "password": "short", "role": "editor"}, nil)
	root.must(201, "POST", "/v1/tenants/acme/apps", map[string]string{"id": "portal", "name": "Portal", "public_domain": "Portal.Acme.test"}, nil)

	var conn struct{ ID, App, Token string }
	root.must(201, "POST", "/v1/apps/portal/connectors", map[string]string{"id": "cn-1"}, &conn)
	if len(conn.Token) < 40 {
		t.Fatalf("connector token: %+v", conn)
	}
	var conns struct{ Items []map[string]any }
	root.must(200, "GET", "/v1/apps/portal/connectors", nil, &conns)
	if len(conns.Items) != 1 || conns.Items[0]["token"] != nil || conns.Items[0]["token_sha256"] != nil {
		t.Fatalf("connector list leaks the token: %v", conns.Items)
	}

	ed, ap, boss := e.client(), e.client(), e.client()
	for c, email := range map[*client]string{ed: "ed@acme.test", ap: "ap@acme.test", boss: "boss@other.test"} {
		c.must(200, "POST", "/v1/auth/login", map[string]string{"email": email, "password": "a-long-password-1"}, nil)
	}

	// Tenant isolation.
	boss.must(404, "GET", "/v1/apps/portal", nil, nil)
	boss.must(403, "POST", "/v1/tenants/acme/apps", map[string]string{"id": "x", "name": "x", "public_domain": "x.test"}, nil)
	var ts struct{ Items []store.Tenant }
	boss.must(200, "GET", "/v1/tenants", nil, &ts)
	if len(ts.Items) != 1 || ts.Items[0].ID != "other" {
		t.Fatalf("tenants seen by another tenant: %+v", ts.Items)
	}

	// Map lifecycle.
	var bad map[string]any
	code := ed.do("POST", "/v1/apps/portal/maps", map[string]any{"document": json.RawMessage(strings.Replace(mapDoc, `"render":"faithful"`, `"render":"nope"`, 1))}, &bad)
	if code != 422 {
		t.Fatalf("invalid map: %d %v", code, bad)
	}
	if bad["details"] == nil {
		t.Errorf("invalid map without details: %v", bad)
	}
	ap.must(403, "POST", "/v1/apps/portal/maps", map[string]any{"document": json.RawMessage(mapDoc)}, nil)
	ed.must(201, "POST", "/v1/apps/portal/maps", map[string]any{"document": json.RawMessage(mapDoc), "note": "first"}, nil)
	ed.must(200, "POST", "/v1/apps/portal/maps/1/submit", nil, nil)
	ed.must(403, "POST", "/v1/apps/portal/maps/1/approve", nil, nil)
	ap.must(409, "POST", "/v1/apps/portal/maps/1/publish", nil, nil) // not approved yet
	ap.must(200, "POST", "/v1/apps/portal/maps/1/approve", nil, nil)
	ap.must(200, "POST", "/v1/apps/portal/maps/1/publish", nil, nil)

	// The author never decides on their own version.
	root.must(201, "POST", "/v1/apps/portal/maps", map[string]any{"document": json.RawMessage(mapDoc)}, nil)
	root.must(200, "POST", "/v1/apps/portal/maps/2/submit", nil, nil)
	root.must(403, "POST", "/v1/apps/portal/maps/2/approve", nil, nil)

	// CSRF header.
	req, _ := http.NewRequest("POST", e.srv.URL+"/v1/apps/portal/maps/2/reject", nil)
	if resp, err := ap.c.Do(req); err != nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("missing CSRF header: %v %v", resp, err)
	}

	// The gateway's view.
	sm := internalGet(t, e, "/v1/internal/apps/portal/map", internalToken)
	var signed struct {
		Version            int
		Payload, Signature string
		KeyID              string `json:"key_id"`
	}
	_ = json.Unmarshal(sm, &signed)
	payload, _ := base64.StdEncoding.DecodeString(signed.Payload)
	sig, _ := base64.StdEncoding.DecodeString(signed.Signature)
	if signed.Version != 1 || signed.KeyID != "k1" || !ed25519.Verify(e.pub, payload, sig) {
		t.Fatalf("signed map: %s", sm)
	}
	var doc struct {
		Meta map[string]any `json:"meta"`
	}
	_ = json.Unmarshal(payload, &doc)
	if doc.Meta["tenant"] != "acme" || doc.Meta["app"] != "portal" || doc.Meta["status"] != "published" ||
		doc.Meta["version"] != float64(1) || doc.Meta["approved_by"] != "user:ap@acme.test" {
		t.Fatalf("published meta: %v", doc.Meta)
	}

	// The relay's view.
	var relay struct{ Connectors []map[string]string }
	_ = json.Unmarshal(internalGet(t, e, "/v1/internal/connectors", internalToken), &relay)
	sum := sha256.Sum256([]byte(conn.Token))
	if len(relay.Connectors) != 1 || relay.Connectors[0]["token_sha256"] != hex.EncodeToString(sum[:]) || relay.Connectors[0]["tenant"] != "acme" {
		t.Fatalf("relay connectors: %v", relay.Connectors)
	}
	if code := internalStatus(t, e, "/v1/internal/connectors", "wrong"); code != http.StatusUnauthorized {
		t.Fatalf("internal without token: %d", code)
	}

	var audit struct{ Items []store.AuditEntry }
	ap.must(200, "GET", "/v1/audit", nil, &audit)
	actions := map[string]bool{}
	for _, a := range audit.Items {
		actions[a.Action] = true
		if a.Tenant == nil || *a.Tenant != "acme" {
			t.Errorf("audit of another tenant: %+v", a)
		}
	}
	for _, want := range []string{"map.draft", "map.submit", "map.approve", "map.publish", "connector.create", "app.create"} {
		if !actions[want] {
			t.Errorf("audit misses %s", want)
		}
	}
	ed.must(403, "GET", "/v1/audit", nil, nil)

	ed.must(204, "POST", "/v1/auth/logout", nil, nil)
	ed.must(401, "GET", "/v1/auth/me", nil, nil)
}

func TestLoginThrottle(t *testing.T) {
	e := setup(t)
	c := e.client()
	for i := 0; i < 5; i++ {
		c.login("root@example.com", "wrong-password-123")
	}
	if code := c.login("root@example.com", "platform-admin-password"); code != http.StatusTooManyRequests {
		t.Fatalf("after 5 failures: %d", code)
	}
}

func internalStatus(t *testing.T, e *env, path, token string) int {
	req, _ := http.NewRequest("GET", e.srv.URL+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := e.srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func internalGet(t *testing.T, e *env, path, token string) []byte {
	t.Helper()
	req, _ := http.NewRequest("GET", e.srv.URL+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := e.srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Fatalf("%s: %d %s", path, resp.StatusCode, b)
	}
	return b
}
