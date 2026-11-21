package api_test

import "testing"

// TestDiscoveryDirectTemplate: the first discovery of a site reached directly
// gives the agent a template whose reach is the chosen one (direct), not the
// template's connector placeholder, so the proposed map is accepted.
func TestDiscoveryDirectTemplate(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	admin := e.join("boss@acme.test")
	admin.must(201, "POST", "/v1/tenants/acme/apps", map[string]any{"id": "shop", "name": "Shop", "public_domain": "shop.acme.test", "kind": "wordpress"}, nil)
	admin.must(201, "POST", "/v1/apps/shop/discoveries", map[string]any{"mode": "direct", "address": "8.8.8.8:443", "use_ai": false}, nil)

	var job struct {
		Template struct {
			Legacy struct {
				Reach map[string]any `json:"reach"`
			} `json:"legacy"`
		} `json:"template"`
	}
	if code := internalPost(t, e, "/v1/internal/discoveries/claim", nil, &job); code != 200 {
		t.Fatalf("claim: %d", code)
	}
	r := job.Template.Legacy.Reach
	if r["mode"] != "direct" || r["address"] != "8.8.8.8:443" || r["connector_id"] != nil {
		t.Fatalf("template reach: %v", r)
	}
}
