package api_test

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestAliases(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	admin := e.join("boss@acme.test")
	admin.must(201, "POST", "/v1/tenants/acme/invitations", map[string]string{"email": "ap@acme.test", "name": "Ap", "role": "viewer"}, nil)
	ap := e.join("ap@acme.test")
	admin.must(201, "POST", "/v1/tenants/acme/connectors", map[string]string{"id": "cn-1"}, nil)
	admin.must(201, "POST", "/v1/tenants/acme/apps", map[string]string{"id": "portal", "name": "Portal", "public_domain": "www.acme.test"}, nil)
	admin.must(201, "POST", "/v1/tenants/acme/apps", map[string]string{"id": "shop", "name": "Shop", "public_domain": "shop.acme.test"}, nil)

	var app struct{ Aliases []string }
	admin.must(200, "PUT", "/v1/apps/portal/aliases", map[string]any{"aliases": []string{"Acme.test.", "acme.test", "old.acme.example"}}, &app)
	if strings.Join(app.Aliases, ",") != "acme.test,old.acme.example" {
		t.Fatalf("aliases: %v", app.Aliases)
	}
	admin.must(409, "PUT", "/v1/apps/shop/aliases", map[string]any{"aliases": []string{"acme.test"}}, nil)
	admin.must(409, "PUT", "/v1/apps/shop/aliases", map[string]any{"aliases": []string{"www.acme.test"}}, nil)
	admin.must(422, "PUT", "/v1/apps/shop/aliases", map[string]any{"aliases": []string{"shop.acme.test"}}, nil)
	admin.must(422, "PUT", "/v1/apps/shop/aliases", map[string]any{"aliases": []string{"not a domain"}}, nil)
	admin.must(409, "POST", "/v1/tenants/acme/apps", map[string]string{"id": "copy", "name": "Copy", "public_domain": "old.acme.example"}, nil)
	ap.must(403, "PUT", "/v1/apps/portal/aliases", map[string]any{"aliases": []string{}}, nil)

	// Published with its aliases; changed aliases apply to the published map.
	admin.must(201, "POST", "/v1/apps/portal/maps", map[string]any{"document": json.RawMessage(mapDoc)}, nil)
	admin.must(200, "POST", "/v1/apps/portal/maps/1/submit", nil, nil)
	admin.must(200, "POST", "/v1/apps/portal/maps/1/approve", nil, nil)
	admin.must(200, "POST", "/v1/apps/portal/maps/1/publish", nil, nil)
	meta := func() map[string]any {
		var signed struct{ Payload string }
		_ = json.Unmarshal(internalGet(t, e, "/v1/internal/apps/portal/map", internalToken), &signed)
		raw, _ := base64.StdEncoding.DecodeString(signed.Payload)
		var doc struct{ Meta map[string]any }
		_ = json.Unmarshal(raw, &doc)
		return doc.Meta
	}
	if got, _ := json.Marshal(meta()["aliases"]); string(got) != `["acme.test","old.acme.example"]` {
		t.Fatalf("published aliases: %s", got)
	}
	admin.must(200, "PUT", "/v1/apps/portal/aliases", map[string]any{"aliases": []string{"acme.test"}}, nil)
	if got, _ := json.Marshal(meta()["aliases"]); string(got) != `["acme.test"]` {
		t.Errorf("aliases not applied to the published map: %s", got)
	}
	if v, _ := meta()["version"].(float64); v != 1 {
		t.Errorf("re-signed as another version: %v", meta()["version"])
	}
}
