package api_test

import (
	"strings"
	"testing"
)

// When the agent has checked a discovery's draft, the tenant's
// administrators get one email: what changes, what came from visitors'
// requests, the link to publish it. None when the kind is turned off.
func TestDraftReadyEmail(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	boss := e.join("boss@acme.test")
	boss.must(201, "POST", "/v1/tenants/acme/connectors", map[string]string{"id": "cn-web01"}, nil)
	boss.must(201, "POST", "/v1/tenants/acme/apps", map[string]any{"id": "blog", "name": "Blog", "public_domain": "blog.acme.test", "kind": "wordpress"}, nil)

	discover := func() {
		t.Helper()
		var d struct{ ID int64 }
		boss.must(201, "POST", "/v1/apps/blog/discoveries", map[string]any{"connector_id": "cn-web01", "address": "127.0.0.1:8443"}, &d)
		var job struct{ Template map[string]any }
		if code := internalPost(t, e, "/v1/internal/discoveries/claim", nil, &job); code != 200 {
			t.Fatalf("claim: %d", code)
		}
		report := map[string]any{"routes_added": []any{map[string]any{"path": "/shop/"}}, "visitors_added": []any{
			map[string]any{"kind": "key", "route": "/shop/", "key": "orderby", "why": "3 visitors used it"}}}
		if code := internalPost(t, e, "/v1/internal/discoveries/"+itoa(d.ID)+"/result",
			map[string]any{"report": report, "document": job.Template}, nil); code != 204 {
			t.Fatalf("result: %d", code)
		}
		var v struct{ ID int64 }
		if code := internalPost(t, e, "/v1/internal/verifications/claim", nil, &v); code != 200 {
			t.Fatalf("verification claim: %d", code)
		}
		score := map[string]any{"functions_total": 4, "functions_identical": 4, "inputs_total": 0, "inputs_protected": 0, "area_percent": 0}
		if code := internalPost(t, e, "/v1/internal/verifications/"+itoa(v.ID)+"/result", map[string]any{"score": score}, nil); code != 204 {
			t.Fatalf("verification result: %d", code)
		}
	}
	ready := func() []string {
		var out []string
		for _, m := range e.mail.Messages() {
			if strings.Contains(m.Subject, "ready to publish") {
				out = append(out, m.To+"\n"+m.Body)
			}
		}
		return out
	}

	discover()
	got := ready()
	if len(got) != 1 || !strings.HasPrefix(got[0], "boss@acme.test\n") {
		t.Fatalf("draft emails: %q", got)
	}
	for _, want := range []string{"from visitors' requests: /shop/ ?orderby= (3 visitors used it)", "4 of 4 pages work",
		"https://panel.test/t/acme/sites/blog/maps/", "Nothing is published until an administrator does"} {
		if !strings.Contains(got[0], want) {
			t.Errorf("email lacks %q:\n%s", want, got[0])
		}
	}

	// turned off in Settings → Alerts: no email
	boss.must(200, "PUT", "/v1/tenants/acme/alerts", map[string]any{"enabled": true, "extra_emails": []string{},
		"kinds": map[string]bool{"draft_ready": false}}, nil)
	discover()
	if n := len(ready()); n != 1 {
		t.Fatalf("emails with draft_ready off: %d", n)
	}
}
