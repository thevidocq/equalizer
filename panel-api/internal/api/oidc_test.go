package api_test

import (
	"strings"
	"testing"
)

func TestOIDCSecret(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	boss := e.join("boss@acme.test")
	boss.must(201, "POST", "/v1/tenants/acme/apps", map[string]string{"id": "portal", "name": "Portal", "public_domain": "portal.acme.test"}, nil)

	var st struct {
		RedirectURI string `json:"redirect_uri"`
		HasSecret   bool   `json:"has_secret"`
	}
	boss.must(200, "GET", "/v1/apps/portal/oidc", nil, &st)
	if st.HasSecret || st.RedirectURI != "https://portal.acme.test/.eq/oidc/callback" {
		t.Fatalf("before: %+v", st)
	}
	boss.must(422, "PUT", "/v1/apps/portal/oidc/secret", map[string]string{"client_secret": " "}, nil)
	boss.must(204, "PUT", "/v1/apps/portal/oidc/secret", map[string]string{"client_secret": "very-secret-value"}, nil)
	boss.must(200, "GET", "/v1/apps/portal/oidc", nil, &st)
	if !st.HasSecret {
		t.Fatal("secret not saved")
	}
	body := string(internalGet(t, e, "/v1/internal/oidc", internalToken))
	if !strings.Contains(body, `"client_secret":"very-secret-value"`) {
		t.Fatalf("internal: %s", body)
	}
	root.must(404, "GET", "/v1/apps/portal/oidc", nil, nil) // the platform does not see it
	boss.must(204, "DELETE", "/v1/apps/portal/oidc/secret", nil, nil)
	if body := string(internalGet(t, e, "/v1/internal/oidc", internalToken)); strings.Contains(body, "very-secret") {
		t.Fatalf("after delete: %s", body)
	}
}
