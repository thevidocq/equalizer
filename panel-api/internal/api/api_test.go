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
	"net/netip"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/api"
	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/dbtest"
	"github.com/thevidocq/equalizer/panel-api/internal/mail"
	"github.com/thevidocq/equalizer/panel-api/internal/mfa"
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
	t      *testing.T
	srv    *httptest.Server
	st     *store.Store
	pub    ed25519.PublicKey
	mail   *mail.Recorder
	server *api.Server
}

func setup(t *testing.T) *env { return setupWith(t, false) }

func testBox() *mfa.Box {
	b, err := mfa.NewBox(make([]byte, 32))
	if err != nil {
		panic(err)
	}
	return b
}

func setupWith(t *testing.T, mfaRequired bool) *env {
	t.Helper()
	if pg == nil {
		t.Skip("PostgreSQL not installed")
	}
	conn := pg.NewDatabase(t)
	st := store.New(conn)
	pub, priv, _ := ed25519.GenerateKey(nil)
	rec := &mail.Recorder{}
	s := &api.Server{
		Store: st, Sessions: &auth.Sessions{DB: conn, TTL: time.Hour}, Throttle: auth.NewThrottle(5, time.Minute),
		Log: slog.New(slog.NewJSONHandler(io.Discard, nil)), SigningKey: priv, KeyID: "k1", InternalToken: internalToken,
		MFA: testBox(), MFARequired: mfaRequired, MFAThrottle: auth.NewThrottle(5, time.Minute),
		Mail: rec, PanelURL: "https://panel.test", ExposureThrottle: auth.NewThrottle(1, time.Minute),
	}
	srv := httptest.NewTLSServer(s.Handler())
	t.Cleanup(srv.Close)
	// Bootstrap the platform administrator as `panel-api create-admin` does.
	hash, _ := auth.HashPassword("platform-admin-password")
	admin := "admin"
	if _, err := st.CreateUser(context.Background(), store.NewUser{Email: "root@example.com", Name: "Root", PasswordHash: hash, PlatformRole: &admin}); err != nil {
		t.Fatal(err)
	}
	return &env{t: t, srv: srv, st: st, pub: pub, mail: rec, server: s}
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
   "render":"faithful","origin":"human"}]}`

// tokenFrom extracts the invitation token from the last email sent to email.
func (e *env) tokenFrom(email string) string {
	e.t.Helper()
	for i := len(e.mail.Sent) - 1; i >= 0; i-- {
		m := e.mail.Sent[i]
		if m.To != email {
			continue
		}
		i := strings.Index(m.Body, "/invite/")
		if i < 0 {
			break
		}
		return strings.Fields(m.Body[i+len("/invite/"):])[0]
	}
	e.t.Fatalf("no invitation for %s", email)
	return ""
}

// join accepts the invitation of email and returns a logged-in client.
func (e *env) join(email string) *client {
	e.t.Helper()
	c := e.client()
	c.must(201, "POST", "/v1/invitations/"+e.tokenFrom(email)+"/accept", map[string]string{"password": "a-long-password-1"}, nil)
	return c
}

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

	// The platform creates tenants; each first administrator gets an email.
	var created struct {
		Invitation struct{ Sent bool }
	}
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, &created)
	if !created.Invitation.Sent || len(e.mail.Sent) != 1 || !strings.Contains(e.mail.Sent[0].Subject, "Acme") {
		t.Fatalf("invitation: %+v %v", created, e.mail.Sent)
	}
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "other", "name": "Other", "admin_email": "boss@other.test", "admin_name": "B"}, nil)
	root.must(409, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Again", "admin_email": "x@x.test", "admin_name": "X"}, nil)

	admin := e.join("boss@acme.test")
	boss := e.join("boss@other.test")
	if code := e.client().do("POST", "/v1/invitations/"+e.tokenFrom("boss@acme.test")+"/accept", map[string]string{"password": "a-long-password-2"}, nil); code != 404 {
		t.Fatalf("invitation used twice: %d", code)
	}

	// Privacy: the platform sees inside no tenant.
	root.must(403, "POST", "/v1/tenants/acme/apps", map[string]string{"id": "x", "name": "x", "public_domain": "x.test"}, nil)
	root.must(403, "GET", "/v1/tenants/acme/users", nil, nil)
	root.must(403, "POST", "/v1/tenants/acme/invitations", map[string]string{"email": "spy@x.test", "name": "Spy", "role": "admin"}, nil)

	admin.must(201, "POST", "/v1/tenants/acme/apps", map[string]string{"id": "portal", "name": "Portal", "public_domain": "Portal.Acme.test"}, nil)
	root.must(404, "GET", "/v1/apps/portal", nil, nil)
	root.must(404, "GET", "/v1/apps/portal/maps", nil, nil)
	root.must(403, "GET", "/v1/tenants/acme/connectors", nil, nil)
	for _, inv := range []struct{ email, role string }{{"ed@acme.test", "editor"}, {"ap@acme.test", "approver"}} {
		admin.must(201, "POST", "/v1/tenants/acme/invitations", map[string]string{"email": inv.email, "name": inv.role, "role": inv.role}, nil)
	}
	var pending struct{ Items []map[string]any }
	admin.must(200, "GET", "/v1/tenants/acme/invitations", nil, &pending)
	if len(pending.Items) != 2 || pending.Items[0]["token"] != nil {
		t.Fatalf("pending invitations: %v", pending.Items)
	}
	ed, ap := e.join("ed@acme.test"), e.join("ap@acme.test")

	var conn struct{ ID, Tenant, Token string }
	admin.must(201, "POST", "/v1/tenants/acme/connectors", map[string]string{"id": "cn-1"}, &conn)
	if len(conn.Token) < 40 {
		t.Fatalf("connector token: %+v", conn)
	}
	var conns struct{ Items []map[string]any }
	ed.must(403, "GET", "/v1/tenants/acme/connectors", nil, nil)
	admin.must(200, "GET", "/v1/tenants/acme/connectors", nil, &conns)
	if len(conns.Items) != 1 || conns.Items[0]["token"] != nil || conns.Items[0]["token_sha256"] != nil {
		t.Fatalf("connector list leaks the token: %v", conns.Items)
	}

	// Tenant isolation.
	boss.must(404, "GET", "/v1/apps/portal", nil, nil)
	boss.must(403, "POST", "/v1/tenants/acme/apps", map[string]string{"id": "x", "name": "x", "public_domain": "x.test"}, nil)
	var ts struct{ Items []store.Tenant }
	boss.must(200, "GET", "/v1/tenants", nil, &ts)
	if len(ts.Items) != 1 || ts.Items[0].ID != "other" {
		t.Fatalf("tenants seen by another tenant: %+v", ts.Items)
	}
	boss.must(403, "GET", "/v1/platform/tenants", nil, nil)

	// Map lifecycle.
	var bad map[string]any
	code := ed.do("POST", "/v1/apps/portal/maps", map[string]any{"document": json.RawMessage(strings.Replace(mapDoc, `"render":"faithful"`, `"render":"nope"`, 1))}, &bad)
	if code != 422 || bad["details"] == nil {
		t.Fatalf("invalid map: %d %v", code, bad)
	}
	ap.must(403, "POST", "/v1/apps/portal/maps", map[string]any{"document": json.RawMessage(mapDoc)}, nil)
	ed.must(201, "POST", "/v1/apps/portal/maps", map[string]any{"document": json.RawMessage(mapDoc), "note": "first"}, nil)
	ed.must(200, "POST", "/v1/apps/portal/maps/1/submit", nil, nil)
	ed.must(403, "POST", "/v1/apps/portal/maps/1/approve", nil, nil)
	ap.must(409, "POST", "/v1/apps/portal/maps/1/publish", nil, nil) // not approved yet
	root.must(404, "POST", "/v1/apps/portal/maps/1/approve", nil, nil)
	ap.must(200, "POST", "/v1/apps/portal/maps/1/approve", nil, nil)
	ap.must(200, "POST", "/v1/apps/portal/maps/1/publish", nil, nil)

	// The author never decides on their own version.
	admin.must(201, "POST", "/v1/apps/portal/maps", map[string]any{"document": json.RawMessage(mapDoc)}, nil)
	admin.must(200, "POST", "/v1/apps/portal/maps/2/submit", nil, nil)
	admin.must(403, "POST", "/v1/apps/portal/maps/2/approve", nil, nil)

	// CSRF header.
	req, _ := http.NewRequest("POST", e.srv.URL+"/v1/apps/portal/maps/2/reject", nil)
	if resp, err := ap.c.Do(req); err != nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("missing CSRF header: %v %v", resp, err)
	}

	// The platform's view: names and status, nothing else.
	var ov struct{ Items []map[string]any }
	root.must(200, "GET", "/v1/platform/tenants", nil, &ov)
	raw, _ := json.Marshal(ov)
	if !strings.Contains(string(raw), `"site_count":1`) || !strings.Contains(string(raw), `"status":"protected"`) ||
		strings.Contains(string(raw), "portal.acme.test") || strings.Contains(string(raw), "cn-1") {
		t.Fatalf("platform overview: %s", raw)
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
		doc.Meta["version"] != float64(1) || doc.Meta["approved_by"] != "user:ap@acme.test" ||
		doc.Meta["public_domain"] != "portal.acme.test" {
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

	// Activity: each side sees only its own.
	var audit struct{ Items []store.AuditEntry }
	ap.must(200, "GET", "/v1/audit", nil, &audit)
	actions := map[string]bool{}
	for _, a := range audit.Items {
		actions[a.Action] = true
		if a.Tenant == nil || *a.Tenant != "acme" {
			t.Errorf("audit of another tenant: %+v", a)
		}
	}
	for _, want := range []string{"map.draft", "map.submit", "map.approve", "map.publish", "connector.create", "app.create", "user.invite", "user.join"} {
		if !actions[want] {
			t.Errorf("audit misses %s", want)
		}
	}
	root.must(200, "GET", "/v1/audit", nil, &audit)
	for _, a := range audit.Items {
		if a.Tenant != nil {
			t.Errorf("platform sees a tenant action: %+v", a)
		}
	}
	ed.must(403, "GET", "/v1/audit", nil, nil)

	ed.must(204, "POST", "/v1/auth/logout", nil, nil)
	ed.must(401, "GET", "/v1/auth/me", nil, nil)
}

func TestInvitationsWithoutMail(t *testing.T) {
	e := setup(t)
	e.server.Mail = mail.Disabled{}
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	var created struct {
		Invitation struct{ Sent bool }
	}
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, &created)
	if created.Invitation.Sent {
		t.Fatal("an unsent invitation reported as sent")
	}
	// The email never arrived: the platform invites again, the old link dies.
	e.server.Mail = e.mail
	root.must(201, "POST", "/v1/platform/tenants/acme/invitation", map[string]string{"admin_email": "giulia@acme.test", "admin_name": "Giulia"}, nil)
	root.must(404, "POST", "/v1/platform/tenants/nope/invitation", map[string]string{"admin_email": "x@acme.test", "admin_name": "X"}, nil)
	admin := e.join("giulia@acme.test")
	var ov struct{ Items []struct{ Joined bool } }
	root.must(200, "GET", "/v1/platform/tenants", nil, &ov)
	if len(ov.Items) != 1 || !ov.Items[0].Joined {
		t.Fatalf("overview after joining: %+v", ov)
	}
	// Once someone joined, only the tenant invites people.
	root.must(409, "POST", "/v1/platform/tenants/acme/invitation", map[string]string{"admin_email": "spy@x.test", "admin_name": "Spy"}, nil)
	admin.must(403, "POST", "/v1/platform/tenants/acme/invitation", map[string]string{"admin_email": "a@x.test", "admin_name": "A"}, nil)

	c := e.client()
	c.must(404, "GET", "/v1/invitations/not-a-token", nil, nil)
	c.must(404, "POST", "/v1/invitations/not-a-token/accept", map[string]string{"password": "a-long-password-1"}, nil)
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

// Behind Caddy every visitor arrives from the proxy: the panel must read
// the visitor's address, or five wrong passwords from anyone would lock
// everyone out.
func TestLoginLimitsPerVisitor(t *testing.T) {
	e := setup(t)
	e.server.TrustedProxies = []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
	login := func(from, password string) int {
		b, _ := json.Marshal(map[string]string{"email": "root@example.com", "password": password})
		req, _ := http.NewRequest("POST", e.srv.URL+"/v1/auth/login", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Eq-Panel", "1")
		req.Header.Set("X-Forwarded-For", "6.6.6.6, "+from) // the client's own value first, the proxy's last
		resp, err := e.client().c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	for i := 0; i < 5; i++ {
		login("198.51.100.7", "wrong-password-123")
	}
	if code := login("198.51.100.7", "platform-admin-password"); code != http.StatusTooManyRequests {
		t.Errorf("the attacker's address after 5 failures: %d", code)
	}
	if code := login("203.0.113.20", "platform-admin-password"); code != http.StatusOK {
		t.Errorf("the real user from another address: %d, want 200", code)
	}
}
