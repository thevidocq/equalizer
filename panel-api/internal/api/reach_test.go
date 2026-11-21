package api_test

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestReachUpdate(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	admin := e.join("boss@acme.test")
	admin.must(201, "POST", "/v1/tenants/acme/invitations", map[string]string{"email": "ap@acme.test", "name": "Ap", "role": "viewer"}, nil)
	ap := e.join("ap@acme.test")
	admin.must(201, "POST", "/v1/tenants/acme/connectors", map[string]string{"id": "cn-1"}, nil)
	admin.must(201, "POST", "/v1/tenants/acme/apps", map[string]string{"id": "portal", "name": "Portal", "public_domain": "www.acme.test"}, nil)

	// No published map yet: cannot change the reach.
	admin.must(409, "PUT", "/v1/apps/portal/reach", map[string]any{"mode": "direct", "address": "8.8.8.8:443"}, nil)

	admin.must(201, "POST", "/v1/apps/portal/maps", map[string]any{"document": json.RawMessage(mapDoc)}, nil)
	admin.must(200, "POST", "/v1/apps/portal/maps/1/submit", nil, nil)
	admin.must(200, "POST", "/v1/apps/portal/maps/1/approve", nil, nil)
	admin.must(200, "POST", "/v1/apps/portal/maps/1/publish", nil, nil)

	reach := func() map[string]any {
		var signed struct{ Payload string }
		_ = json.Unmarshal(internalGet(t, e, "/v1/internal/apps/portal/map", internalToken), &signed)
		raw, _ := base64.StdEncoding.DecodeString(signed.Payload)
		var doc struct {
			Legacy struct {
				Reach map[string]any `json:"reach"`
			} `json:"legacy"`
		}
		_ = json.Unmarshal(raw, &doc)
		return doc.Legacy.Reach
	}

	// Switch to direct; the published map carries the new reach, same version.
	var app struct {
		ReachMode    *string `json:"reach_mode"`
		ReachAddress *string `json:"reach_address"`
	}
	admin.must(200, "PUT", "/v1/apps/portal/reach", map[string]any{"mode": "direct", "address": "8.8.8.8:443"}, &app)
	if app.ReachMode == nil || *app.ReachMode != "direct" || app.ReachAddress == nil || *app.ReachAddress != "8.8.8.8:443" {
		t.Fatalf("app reach after switch: %+v", app)
	}
	if r := reach(); r["mode"] != "direct" || r["address"] != "8.8.8.8:443" {
		t.Fatalf("published reach: %v", r)
	}

	// Back to the connector.
	admin.must(200, "PUT", "/v1/apps/portal/reach", map[string]any{"mode": "connector", "connector_id": "cn-1", "address": "10.0.0.1:8080"}, nil)
	if r := reach(); r["mode"] != "connector" || r["connector_id"] != "cn-1" {
		t.Fatalf("published reach back to connector: %v", r)
	}

	// An unknown connector is refused; a viewer cannot change the reach.
	admin.must(409, "PUT", "/v1/apps/portal/reach", map[string]any{"mode": "connector", "connector_id": "cn-nope", "address": "10.0.0.1:8080"}, nil)
	ap.must(403, "PUT", "/v1/apps/portal/reach", map[string]any{"mode": "direct", "address": "8.8.8.8:443"}, nil)
}
