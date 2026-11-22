package api_test

import "testing"

// Distinct visitors are counted from the gateway's visitor keys: one visitor
// coming back in later reports is one, and a malformed key is not a visitor.
func TestUnmappedDistinctVisitors(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	boss := e.join("boss@acme.test")
	boss.must(201, "POST", "/v1/tenants/acme/connectors", map[string]string{"id": "cn-web01"}, nil)
	boss.must(201, "POST", "/v1/tenants/acme/apps", map[string]any{"id": "blog", "name": "Blog", "public_domain": "blog.acme.test", "kind": "wordpress"}, nil)

	send := func(path string, keys ...string) {
		t.Helper()
		item := map[string]any{"app": "blog", "method": "GET", "path": path, "hits": 1, "clients": len(keys), "client_keys": keys}
		if code := internalPost(t, e, "/v1/internal/unmapped", map[string]any{"items": []any{item}}, nil); code != 204 {
			t.Fatalf("unmapped: %d", code)
		}
	}
	for i := 0; i < 5; i++ { // the same visitor, five minutes in a row
		send("/once.pdf", "00112233aabbccdd")
	}
	send("/twice.pdf", "00112233aabbccdd")
	send("/twice.pdf", "ffeeddccbbaa9988", "not-a-key")

	boss.must(201, "POST", "/v1/apps/blog/discoveries", map[string]any{"connector_id": "cn-web01", "address": "127.0.0.1:8443"}, nil)
	var job struct {
		Unmapped []struct {
			Path    string
			Hits    int
			Clients int
		}
	}
	if code := internalPost(t, e, "/v1/internal/discoveries/claim", nil, &job); code != 200 {
		t.Fatalf("claim: %d", code)
	}
	got := map[string]int{}
	for _, u := range job.Unmapped {
		got[u.Path] = u.Clients
	}
	if got["/once.pdf"] != 1 || got["/twice.pdf"] != 2 || len(job.Unmapped) != 2 || job.Unmapped[0].Path != "/twice.pdf" {
		t.Fatalf("visitors: %+v", job.Unmapped)
	}
}
