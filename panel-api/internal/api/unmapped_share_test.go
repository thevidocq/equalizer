package api_test

import (
	"fmt"
	"testing"
)

// On a public site scanners fill the refused paths with one-off requests: a
// query key a single visitor used still reaches the discovery.
func TestDroppedKeysNotCrowdedOut(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	boss := e.join("boss@acme.test")
	boss.must(201, "POST", "/v1/tenants/acme/connectors", map[string]string{"id": "cn-web01"}, nil)
	boss.must(201, "POST", "/v1/tenants/acme/apps", map[string]any{"id": "blog", "name": "Blog", "public_domain": "blog.acme.test", "kind": "wordpress"}, nil)

	items := []map[string]any{}
	for i := 0; i < 300; i++ {
		items = append(items, map[string]any{"app": "blog", "method": "GET", "path": fmt.Sprintf("/probe-%03d.php", i), "hits": 2, "clients": 1})
	}
	items = append(items, map[string]any{"app": "blog", "method": "GET", "path": "/?prova=", "hits": 1, "clients": 1})
	if code := internalPost(t, e, "/v1/internal/unmapped", map[string]any{"items": items}, nil); code != 204 {
		t.Fatalf("unmapped: %d", code)
	}
	boss.must(201, "POST", "/v1/apps/blog/discoveries", map[string]any{"connector_id": "cn-web01", "address": "127.0.0.1:8443"}, nil)
	var job struct{ Unmapped []map[string]any }
	if code := internalPost(t, e, "/v1/internal/discoveries/claim", nil, &job); code != 200 {
		t.Fatalf("claim: %d", code)
	}
	found := false
	for _, u := range job.Unmapped {
		found = found || u["path"] == "/?prova="
	}
	if !found || len(job.Unmapped) != 201 {
		t.Fatalf("job has %d reports, /?prova= present: %v", len(job.Unmapped), found)
	}
}
