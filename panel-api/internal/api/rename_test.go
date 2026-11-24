package api_test

import "testing"

func TestRenameSite(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	boss := e.join("boss@acme.test")
	boss.must(201, "POST", "/v1/tenants/acme/invitations", map[string]string{"email": "ap@acme.test", "name": "Ap", "role": "viewer"}, nil)
	viewer := e.join("ap@acme.test")
	boss.must(201, "POST", "/v1/tenants/acme/apps", map[string]any{"id": "blog", "name": "Blog", "public_domain": "blog.acme.test"}, nil)

	viewer.must(403, "PATCH", "/v1/apps/blog", map[string]string{"name": "Mine"}, nil)
	boss.must(422, "PATCH", "/v1/apps/blog", map[string]string{"name": "   "}, nil)
	var a struct{ ID, Name, PublicDomain string }
	boss.must(200, "PATCH", "/v1/apps/blog", map[string]string{"name": "  Company blog "}, &a)
	if a.ID != "blog" || a.Name != "Company blog" {
		t.Fatalf("renamed: %+v", a)
	}
	boss.must(200, "GET", "/v1/apps/blog", nil, &a)
	if a.Name != "Company blog" {
		t.Fatalf("after: %+v", a)
	}
	// with another change in the same request
	boss.must(200, "PATCH", "/v1/apps/blog", map[string]any{"name": "Blog", "never_public": true}, &a)
	if a.Name != "Blog" {
		t.Fatalf("both: %+v", a)
	}
}
