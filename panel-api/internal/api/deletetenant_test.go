package api_test

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDeleteTenant(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "other", "name": "Other", "admin_email": "o@other.test", "admin_name": "O"}, nil)
	boss := e.join("boss@acme.test")
	boss.must(201, "POST", "/v1/tenants/acme/invitations", map[string]string{"email": "vi@acme.test", "name": "Vi", "role": "viewer"}, nil)
	boss.must(201, "POST", "/v1/tenants/acme/connectors", map[string]string{"id": "cn-1"}, nil)
	boss.must(201, "POST", "/v1/tenants/acme/apps", map[string]string{"id": "portal", "name": "Portal", "public_domain": "portal.acme.test"}, nil)
	boss.must(201, "POST", "/v1/apps/portal/maps", map[string]any{"document": json.RawMessage(mapDoc)}, nil)
	boss.must(200, "POST", "/v1/apps/portal/maps/1/publish", nil, nil)
	boss.must(201, "POST", "/v1/apps/portal/maps", map[string]any{"document": json.RawMessage(mapDoc)}, nil)

	boss.must(403, "DELETE", "/v1/platform/tenants/acme", map[string]string{"confirm": "acme"}, nil)
	root.must(422, "DELETE", "/v1/platform/tenants/acme", map[string]string{"confirm": "Acme"}, nil)
	root.must(404, "DELETE", "/v1/platform/tenants/nobody", map[string]string{"confirm": "nobody"}, nil)
	root.must(204, "DELETE", "/v1/platform/tenants/acme", map[string]string{"confirm": "acme"}, nil)

	var ov struct{ Items []struct{ ID string } }
	root.must(200, "GET", "/v1/platform/tenants", nil, &ov)
	if len(ov.Items) != 1 || ov.Items[0].ID != "other" {
		t.Fatalf("after delete: %+v", ov)
	}
	if maps := string(internalGet(t, e, "/v1/internal/maps", internalToken)); strings.Contains(maps, "portal") {
		t.Fatalf("the gateways still get the map: %s", maps)
	}
	if cs := string(internalGet(t, e, "/v1/internal/connectors", internalToken)); strings.Contains(cs, "cn-1") {
		t.Fatalf("the relays still accept the connector: %s", cs)
	}
	boss.must(401, "GET", "/v1/auth/me", nil, nil)
	if e.client().login("boss@acme.test", "a-long-password-1") != 401 {
		t.Fatal("the tenant's people can still log in")
	}
	// The same id and the same people can start again from scratch.
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	var audit struct {
		Items []struct{ Action, Target string }
	}
	root.must(200, "GET", "/v1/audit", nil, &audit)
	found := false
	for _, a := range audit.Items {
		found = found || (a.Action == "tenant.delete" && a.Target == "acme")
	}
	if !found {
		t.Fatalf("platform activity misses the deletion: %+v", audit.Items)
	}
}
