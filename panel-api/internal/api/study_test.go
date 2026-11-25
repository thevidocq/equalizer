package api_test

import (
	"encoding/json"
	"testing"
)

// The security study waits for the check of the discovery's draft, is
// written on the map as the check left it (the corrected version), and
// becomes the report's security section. Without a draft it is taken at
// once, on the published map.
func TestSecurityStudyAfterTheCheck(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	boss := e.join("boss@acme.test")
	boss.must(201, "POST", "/v1/tenants/acme/connectors", map[string]string{"id": "cn-web01"}, nil)
	boss.must(201, "POST", "/v1/tenants/acme/apps", map[string]any{"id": "blog", "name": "Blog", "public_domain": "blog.acme.test", "kind": "wordpress"}, nil)
	boss.must(200, "PUT", "/v1/tenants/acme/llm", map[string]any{"provider": "anthropic", "model": "claude-haiku-5-5", "api_key": "sk-test"}, nil)

	var d struct{ ID int64 }
	boss.must(201, "POST", "/v1/apps/blog/discoveries", map[string]any{"connector_id": "cn-web01", "address": "127.0.0.1:8443", "use_ai": true}, &d)
	var job struct{ Template map[string]any }
	if code := internalPost(t, e, "/v1/internal/discoveries/claim", nil, &job); code != 200 {
		t.Fatalf("claim: %d", code)
	}
	report := map[string]any{"cms": "wordpress", "security": map[string]any{"used": false, "pending": true}}
	if code := internalPost(t, e, "/v1/internal/discoveries/"+itoa(d.ID)+"/result",
		map[string]any{"report": report, "document": job.Template}, nil); code != 204 {
		t.Fatalf("result: %d", code)
	}
	// the check of the draft is not over: no study yet
	if code := internalPost(t, e, "/v1/internal/studies/claim", nil, nil); code != 204 {
		t.Fatalf("a study taken before the check: %d", code)
	}
	var v struct{ ID int64 }
	if code := internalPost(t, e, "/v1/internal/verifications/claim", nil, &v); code != 200 {
		t.Fatalf("verification claim: %d", code)
	}
	score := map[string]any{"functions_total": 4, "functions_identical": 4, "inputs_total": 0, "inputs_protected": 0, "area_percent": 0}
	if code := internalPost(t, e, "/v1/internal/verifications/"+itoa(v.ID)+"/result",
		map[string]any{"score": score, "document": job.Template, "before": score, "corrections": []any{}}, nil); code != 204 {
		t.Fatalf("verification result: %d", code)
	}
	var sj struct {
		ID       int64
		Version  int
		Report   map[string]any
		Document map[string]any
		LLM      map[string]any
	}
	if code := internalPost(t, e, "/v1/internal/studies/claim", nil, &sj); code != 200 {
		t.Fatalf("study claim: %d", code)
	}
	var disc struct {
		DraftVersion *int `json:"draft_version"`
	}
	boss.must(200, "GET", "/v1/apps/blog/discoveries/"+itoa(d.ID), nil, &disc)
	if sj.ID != d.ID || disc.DraftVersion == nil || sj.Version != *disc.DraftVersion || sj.Report["cms"] != "wordpress" ||
		sj.Document["legacy"] == nil || sj.LLM["api_key"] != "sk-test" {
		t.Fatalf("job: %+v (draft %v)", sj, disc.DraftVersion)
	}
	if code := internalPost(t, e, "/v1/internal/studies/claim", nil, nil); code != 204 {
		t.Fatalf("a study taken twice: %d", code)
	}
	if code := internalPost(t, e, "/v1/internal/studies/"+itoa(d.ID)+"/result",
		map[string]any{"security": map[string]any{"bad": 1}}, nil); code != 422 {
		t.Fatalf("a study without used: %d", code)
	}
	study := map[string]any{"used": true, "version": sj.Version, "summary": "A blog.", "findings": []any{}}
	if code := internalPost(t, e, "/v1/internal/studies/"+itoa(d.ID)+"/result", map[string]any{"security": study}, nil); code != 204 {
		t.Fatalf("study result: %d", code)
	}
	var got struct {
		Report struct {
			CMS      string          `json:"cms"`
			Security json.RawMessage `json:"security"`
		}
	}
	boss.must(200, "GET", "/v1/apps/blog/discoveries/"+itoa(d.ID), nil, &got)
	var sec map[string]any
	_ = json.Unmarshal(got.Report.Security, &sec)
	if got.Report.CMS != "wordpress" || sec["summary"] != "A blog." || sec["pending"] != nil {
		t.Fatalf("report: %+v", got.Report)
	}
	if code := internalPost(t, e, "/v1/internal/studies/"+itoa(d.ID)+"/result", map[string]any{"security": study}, nil); code != 404 {
		t.Fatalf("a study written twice: %d", code)
	}
}
