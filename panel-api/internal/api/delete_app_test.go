package api_test

import (
	"encoding/json"
	"testing"
)

func TestDeleteApp(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	admin := e.join("boss@acme.test")
	admin.must(201, "POST", "/v1/tenants/acme/invitations", map[string]string{"email": "ap@acme.test", "name": "Ap", "role": "viewer"}, nil)
	ap := e.join("ap@acme.test")
	admin.must(201, "POST", "/v1/tenants/acme/connectors", map[string]string{"id": "cn-1"}, nil)
	admin.must(201, "POST", "/v1/tenants/acme/apps", map[string]string{"id": "portal", "name": "Portal", "public_domain": "www.acme.test"}, nil)
	admin.must(201, "POST", "/v1/apps/portal/maps", map[string]any{"document": json.RawMessage(mapDoc)}, nil)

	// A viewer cannot delete the site.
	ap.must(403, "DELETE", "/v1/apps/portal", nil, nil)
	// The site and its map are still there.
	admin.must(200, "GET", "/v1/apps/portal", nil, nil)

	// The administrator deletes it; everything belonging to it goes too.
	admin.must(204, "DELETE", "/v1/apps/portal", nil, nil)
	admin.must(404, "GET", "/v1/apps/portal", nil, nil)
	// The id frees up: a new site can take it.
	admin.must(201, "POST", "/v1/tenants/acme/apps", map[string]string{"id": "portal", "name": "Portal 2", "public_domain": "www.acme.test"}, nil)
	// Deleting a site that does not exist is a 404.
	admin.must(404, "DELETE", "/v1/apps/nope", nil, nil)
}
