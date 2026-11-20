package api_test

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReview(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	boss := e.join("boss@acme.test")
	boss.must(201, "POST", "/v1/tenants/acme/invitations", map[string]string{"email": "ap@acme.test", "name": "Ap", "role": "approver"}, nil)
	ap := e.join("ap@acme.test")
	boss.must(201, "POST", "/v1/tenants/acme/connectors", map[string]string{"id": "cn-1"}, nil)
	boss.must(201, "POST", "/v1/tenants/acme/apps", map[string]string{"id": "portal", "name": "Portal", "public_domain": "portal.acme.test"}, nil)
	boss.must(201, "POST", "/v1/apps/portal/maps", map[string]any{"document": json.RawMessage(mapDoc)}, nil)
	for _, a := range []string{"submit"} {
		boss.must(200, "POST", "/v1/apps/portal/maps/1/"+a, nil, nil)
	}
	ap.must(200, "POST", "/v1/apps/portal/maps/1/approve", nil, nil)
	ap.must(200, "POST", "/v1/apps/portal/maps/1/publish", nil, nil)

	// Version 2 opens a backup path and the home page gets a free-text query.
	next := strings.Replace(mapDoc, `"routes":[`, `"routes":[{"id":"bk","path":"/backup/{f}","method":"GET","roles":["guest"],"params":{"f":{"type":"string","max_length":50}},"legacy_request":{"method":"GET","path":"/bk/{{params.f}}"},"render":"faithful","origin":"human"},`, 1)
	boss.must(201, "POST", "/v1/apps/portal/maps", map[string]any{"document": json.RawMessage(next)}, nil)

	var rv struct {
		BaseVersion *int `json:"base_version"`
		Changes     []struct{ Entry, Change string }
		Findings    []struct{ Severity, Rule, Entry string }
		AI          struct{ Status string }
	}
	ap.must(200, "GET", "/v1/apps/portal/maps/2/review", nil, &rv)
	if rv.BaseVersion == nil || *rv.BaseVersion != 1 || rv.AI.Status != "off" {
		t.Fatalf("review: %+v", rv)
	}
	if len(rv.Findings) == 0 || rv.Findings[0].Rule != "sensitive_path" || rv.Findings[0].Severity != "high" {
		t.Fatalf("findings: %+v", rv.Findings)
	}
	ap.must(409, "POST", "/v1/apps/portal/maps/2/review", nil, nil) // no AI configured

	// With the tenant's model, submitting queues the AI review for the agent.
	boss.must(200, "PUT", "/v1/tenants/acme/llm", map[string]any{"provider": "anthropic", "model": "m", "api_key": "sk-1"}, nil)
	boss.must(200, "POST", "/v1/apps/portal/maps/2/submit", nil, nil)
	var job struct {
		ID     int64
		Review struct{ Findings []map[string]any }
		LLM    map[string]any
	}
	if code := internalPost(t, e, "/v1/internal/reviews/claim", nil, &job); code != 200 || job.LLM["api_key"] != "sk-1" || len(job.Review.Findings) == 0 {
		t.Fatalf("claim review: %d %+v", code, job)
	}
	ap.must(200, "GET", "/v1/apps/portal/maps/2/review", nil, &rv)
	if rv.AI.Status != "running" {
		t.Fatalf("while the agent works: %+v", rv.AI)
	}
	internalPost(t, e, "/v1/internal/reviews/"+itoa(job.ID)+"/result", map[string]any{"summary": "Adds /backup/{f}.", "questions": []string{"Why expose backups?"}}, nil)
	var done struct {
		AI struct {
			Status, Summary string
			Questions       []string
		}
	}
	ap.must(200, "GET", "/v1/apps/portal/maps/2/review", nil, &done)
	if done.AI.Status != "done" || done.AI.Summary != "Adds /backup/{f}." || len(done.AI.Questions) != 1 {
		t.Fatalf("AI review: %+v", done.AI)
	}
	ap.must(202, "POST", "/v1/apps/portal/maps/2/review", nil, nil)
	ap.must(409, "POST", "/v1/apps/portal/maps/2/review", nil, nil)
	e.client().must(401, "GET", "/v1/apps/portal/maps/2/review", nil, nil)
}

func TestRollback(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	boss := e.join("boss@acme.test")
	boss.must(201, "POST", "/v1/tenants/acme/invitations", map[string]string{"email": "ap@acme.test", "name": "Ap", "role": "approver"}, nil)
	boss.must(201, "POST", "/v1/tenants/acme/invitations", map[string]string{"email": "ed@acme.test", "name": "Ed", "role": "editor"}, nil)
	ap, ed := e.join("ap@acme.test"), e.join("ed@acme.test")
	boss.must(201, "POST", "/v1/tenants/acme/connectors", map[string]string{"id": "cn-1"}, nil)
	boss.must(201, "POST", "/v1/tenants/acme/apps", map[string]string{"id": "portal", "name": "Portal", "public_domain": "portal.acme.test"}, nil)
	for v := 1; v <= 2; v++ {
		ed.must(201, "POST", "/v1/apps/portal/maps", map[string]any{"document": json.RawMessage(mapDoc)}, nil)
		ed.must(200, "POST", "/v1/apps/portal/maps/"+itoa(int64(v))+"/submit", nil, nil)
		ap.must(200, "POST", "/v1/apps/portal/maps/"+itoa(int64(v))+"/approve", nil, nil)
		ap.must(200, "POST", "/v1/apps/portal/maps/"+itoa(int64(v))+"/publish", nil, nil)
	}
	ed.must(403, "POST", "/v1/apps/portal/maps/1/rollback", nil, nil)
	ap.must(409, "POST", "/v1/apps/portal/maps/2/rollback", nil, nil) // already published
	var v struct{ Status string }
	ap.must(200, "POST", "/v1/apps/portal/maps/1/rollback", nil, &v)
	if v.Status != "published" {
		t.Fatalf("rolled back: %+v", v)
	}
	var signed struct{ Version int }
	_ = json.Unmarshal(internalGet(t, e, "/v1/internal/apps/portal/map", internalToken), &signed)
	if signed.Version != 1 {
		t.Fatalf("the gateway gets version %d", signed.Version)
	}
	var audit struct{ Items []struct{ Action string } }
	ap.must(200, "GET", "/v1/audit", nil, &audit)
	if audit.Items[0].Action != "map.rollback" {
		t.Fatalf("audit: %+v", audit.Items[0])
	}
}
