package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func internalPost(t *testing.T, e *env, path string, body any, out any) int {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", e.srv.URL+path, strings.NewReader(string(b)))
	req.Header.Set("Authorization", "Bearer "+internalToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil && resp.StatusCode == 200 {
		_ = json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode
}

func TestDiscovery(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	boss := e.join("boss@acme.test")
	boss.must(201, "POST", "/v1/tenants/acme/connectors", map[string]string{"id": "cn-web01"}, nil)
	boss.must(201, "POST", "/v1/tenants/acme/apps", map[string]any{"id": "blog", "name": "Blog", "public_domain": "blog.acme.test", "kind": "wordpress"}, nil)

	// The tenant's AI: the key is never returned, the platform cannot touch it.
	root.must(403, "GET", "/v1/tenants/acme/llm", nil, nil)
	boss.must(422, "PUT", "/v1/tenants/acme/llm", map[string]any{"provider": "anthropic", "model": "claude-haiku-5-5"}, nil)
	boss.must(422, "PUT", "/v1/tenants/acme/llm", map[string]any{"provider": "local", "model": "llama", "endpoint": "http://10.0.0.1/v1"}, nil)
	var cfg map[string]any
	boss.must(200, "PUT", "/v1/tenants/acme/llm", map[string]any{"provider": "anthropic", "model": "claude-haiku-5-5", "api_key": "sk-test-123"}, &cfg)
	raw, _ := json.Marshal(cfg)
	if cfg["has_api_key"] != true || strings.Contains(string(raw), "sk-test") {
		t.Fatalf("llm config: %s", raw)
	}

	// Without a published map, connector and address are needed.
	boss.must(422, "POST", "/v1/apps/blog/discoveries", map[string]any{}, nil)
	boss.must(422, "POST", "/v1/apps/blog/discoveries", map[string]any{"connector_id": "cn-other", "address": "127.0.0.1:8443"}, nil)
	var d struct {
		ID          int64
		UseAI       bool `json:"use_ai"`
		WithAccount bool `json:"with_account"`
	}
	boss.must(201, "POST", "/v1/apps/blog/discoveries", map[string]any{"connector_id": "cn-web01", "address": "127.0.0.1:8443",
		"account": map[string]string{"username": "tester", "password": "s3cret!"}}, &d)
	if !d.UseAI || !d.WithAccount {
		t.Fatalf("discovery: %+v", d)
	}
	boss.must(409, "POST", "/v1/apps/blog/discoveries", map[string]any{"connector_id": "cn-web01", "address": "127.0.0.1:8443"}, nil)

	// Gateway reports.
	if code := internalPost(t, e, "/v1/internal/unmapped", map[string]any{"items": []map[string]any{
		{"app": "blog", "method": "GET", "path": "/events/", "hits": 12, "clients": 7},
		{"app": "nope", "method": "GET", "path": "/x", "hits": 1, "clients": 1}}}, nil); code != 204 {
		t.Fatalf("unmapped: %d", code)
	}

	// The agent takes the job: everything it needs, secrets included.
	var job struct {
		ID       int64
		BaseURL  string `json:"base_url"`
		Template map[string]any
		Account  map[string]string
		LLM      map[string]any
		Unmapped []map[string]any
	}
	if code := internalPost(t, e, "/v1/internal/discoveries/claim", nil, &job); code != 200 {
		t.Fatalf("claim: %d", code)
	}
	if job.ID != d.ID || job.BaseURL != "https://blog.acme.test" || job.Template == nil || job.Account["password"] != "s3cret!" ||
		job.LLM["api_key"] != "sk-test-123" || len(job.Unmapped) != 1 {
		t.Fatalf("job: %+v", job)
	}
	if code := internalPost(t, e, "/v1/internal/discoveries/claim", nil, nil); code != 204 {
		t.Fatalf("second claim: %d", code)
	}
	internalPost(t, e, "/v1/internal/discoveries/"+itoa(d.ID)+"/progress", map[string]any{"pages": 40}, nil)

	// An invalid proposal fails the discovery; a valid one becomes a draft by the requester.
	if code := internalPost(t, e, "/v1/internal/discoveries/"+itoa(d.ID)+"/result",
		map[string]any{"report": map[string]any{"cms": "wordpress"}, "document": job.Template}, nil); code != 204 {
		t.Fatalf("result: %d", code)
	}
	var done struct {
		Status       string
		DraftVersion *int `json:"draft_version"`
		Report       map[string]any
	}
	boss.must(200, "GET", "/v1/apps/blog/discoveries/"+itoa(d.ID), nil, &done)
	if done.Status != "done" || done.DraftVersion == nil || *done.DraftVersion != 1 || done.Report["cms"] != "wordpress" {
		t.Fatalf("finished: %+v", done)
	}
	var ver struct{ Note, Author string }
	boss.must(200, "GET", "/v1/apps/blog/maps/1", nil, &ver)
	if !strings.Contains(ver.Note, "discovery") || ver.Author != "boss@acme.test" {
		t.Fatalf("draft: %+v", ver)
	}
	var sealed int
	_ = e.st.DB.QueryRowContext(context.Background(), `SELECT count(*) FROM discoveries WHERE account_secret IS NOT NULL`).Scan(&sealed)
	if sealed != 0 {
		t.Fatal("the test account password was kept after the discovery")
	}

	// Continuous discovery needs a published map.
	boss.must(200, "PATCH", "/v1/apps/blog", map[string]any{"discovery_schedule": "daily"}, nil)
	if n, err := e.server.ScheduleDiscoveries(context.Background()); err != nil || n != 0 {
		t.Fatalf("scheduled without a published map: %d %v", n, err)
	}
	boss.must(422, "PATCH", "/v1/apps/blog", map[string]any{"discovery_schedule": "hourly"}, nil)
}
