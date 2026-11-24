package api_test

import "testing"

// A running verification says where it is; the end erases it.
func TestVerificationProgress(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	boss := e.join("boss@acme.test")
	boss.must(201, "POST", "/v1/tenants/acme/connectors", map[string]string{"id": "cn-web01"}, nil)
	boss.must(201, "POST", "/v1/tenants/acme/apps", map[string]any{"id": "blog", "name": "Blog", "public_domain": "blog.acme.test", "kind": "wordpress"}, nil)
	var d struct{ ID int64 }
	boss.must(201, "POST", "/v1/apps/blog/discoveries", map[string]any{"connector_id": "cn-web01", "address": "127.0.0.1:8443"}, &d)
	var job struct{ Template map[string]any }
	internalPost(t, e, "/v1/internal/discoveries/claim", nil, &job)
	internalPost(t, e, "/v1/internal/discoveries/"+itoa(d.ID)+"/result", map[string]any{"report": map[string]any{}, "document": job.Template}, nil)
	var v struct {
		ID      int64
		Version int
	}
	if internalPost(t, e, "/v1/internal/verifications/claim", nil, &v) != 200 {
		t.Fatal("claim")
	}
	if c := internalPost(t, e, "/v1/internal/verifications/"+itoa(v.ID)+"/progress", map[string]any{"step": "pages", "done": 5, "total": 3}, nil); c != 422 {
		t.Fatalf("done over total: %d", c)
	}
	if c := internalPost(t, e, "/v1/internal/verifications/"+itoa(v.ID)+"/progress", map[string]any{"step": "pages", "done": 120, "total": 349}, nil); c != 204 {
		t.Fatalf("progress: %d", c)
	}
	var got struct {
		Status   string
		Progress *struct {
			Step        string
			Done, Total int
		}
	}
	boss.must(200, "GET", "/v1/apps/blog/maps/"+itoa(int64(v.Version))+"/verify", nil, &got)
	if got.Status != "running" || got.Progress == nil || got.Progress.Done != 120 || got.Progress.Total != 349 {
		t.Fatalf("running: %+v", got)
	}
	internalPost(t, e, "/v1/internal/verifications/"+itoa(v.ID)+"/result", map[string]any{"score": map[string]any{
		"functions_total": 1, "functions_identical": 1, "inputs_total": 0, "inputs_protected": 0, "area_percent": 0}}, nil)
	boss.must(200, "GET", "/v1/apps/blog/maps/"+itoa(int64(v.Version))+"/verify", nil, &got)
	if got.Status != "done" || got.Progress != nil {
		t.Fatalf("done: %+v", got)
	}
}
