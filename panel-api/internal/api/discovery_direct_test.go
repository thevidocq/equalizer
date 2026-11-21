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

// TestDiscoveryLegacyHost: the legacy site is asked for the name it answers
// to. A direct IP keeps the public domain; the advanced override replaces it
// (a parallel public domain in front of a site behind a CDN).
func TestDiscoveryLegacyHost(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	admin := e.join("boss@acme.test")
	admin.must(201, "POST", "/v1/tenants/acme/apps", map[string]any{"id": "ip", "name": "Ip", "public_domain": "ip.acme.test", "kind": "wordpress"}, nil)
	admin.must(201, "POST", "/v1/tenants/acme/apps", map[string]any{"id": "par", "name": "Par", "public_domain": "metreta.acme.test", "kind": "wordpress"}, nil)

	type job struct {
		App     string `json:"app"`
		BaseURL string `json:"base_url"`
	}
	claim := func() job {
		var j job
		if code := internalPost(t, e, "/v1/internal/discoveries/claim", nil, &j); code != 200 {
			t.Fatalf("claim: %d", code)
		}
		return j
	}
	admin.must(422, "POST", "/v1/apps/par/discoveries", map[string]any{"mode": "direct", "address": "8.8.8.8:443", "legacy_host": "bad host!", "use_ai": false}, nil)
	admin.must(201, "POST", "/v1/apps/ip/discoveries", map[string]any{"mode": "direct", "address": "8.8.8.8:443", "use_ai": false}, nil)
	if j := claim(); j.BaseURL != "https://ip.acme.test" {
		t.Fatalf("direct IP: %+v", j)
	}
	admin.must(201, "POST", "/v1/apps/par/discoveries", map[string]any{"mode": "direct", "address": "8.8.8.8:443", "legacy_host": "www.metreta.com", "use_ai": false}, nil)
	if j := claim(); j.BaseURL != "https://www.metreta.com" {
		t.Fatalf("override: %+v", j)
	}
}
