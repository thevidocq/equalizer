package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// A version is exported as OpenAPI 3 or ModSecurity rules, by whoever can
// read it; other tenants never.
func TestExportMap(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "other", "name": "Other", "admin_email": "o@other.test", "admin_name": "O"}, nil)
	boss := e.join("boss@acme.test")
	boss.must(201, "POST", "/v1/tenants/acme/connectors", map[string]string{"id": "cn-1"}, nil)
	boss.must(201, "POST", "/v1/tenants/acme/apps", map[string]string{"id": "portal", "name": "Portal", "public_domain": "portal.acme.test"}, nil)
	boss.must(201, "POST", "/v1/apps/portal/maps", map[string]any{"document": json.RawMessage(mapDoc)}, nil)

	get := func(c *client, format string) (int, http.Header, string) {
		req, _ := http.NewRequest("GET", e.srv.URL+"/v1/apps/portal/maps/1/export?format="+format, nil)
		req.Header.Set("X-Eq-Panel", "1")
		resp, err := c.c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, resp.Header, string(b)
	}
	code, h, body := get(boss, "openapi")
	if code != 200 || !strings.Contains(h.Get("Content-Disposition"), `portal-map-v1.openapi.json`) ||
		!strings.Contains(body, `"openapi": "3.0.3"`) || !strings.Contains(body, "https://portal.acme.test") {
		t.Fatalf("openapi: %d %v %s", code, h, body)
	}
	code, h, body = get(boss, "modsecurity")
	if code != 200 || !strings.HasPrefix(h.Get("Content-Type"), "text/plain") || !strings.Contains(body, `setvar:'tx.eq_entry=r_home'`) {
		t.Fatalf("modsecurity: %d %v %s", code, h, body)
	}
	if code, _, _ = get(boss, "f5"); code != 422 {
		t.Fatalf("an unknown format: %d", code)
	}
	o := e.join("o@other.test")
	if code, _, _ = get(o, "openapi"); code != 404 && code != 403 {
		t.Fatalf("another tenant: %d", code)
	}
}
