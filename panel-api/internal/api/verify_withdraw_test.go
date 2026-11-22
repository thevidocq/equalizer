package api_test

import (
	"context"
	"encoding/json"
	"testing"
)

// After a discovery, a draft a person submitted while the agent verified it
// is theirs: no corrected draft appears, nothing is retired, and the
// discovery keeps proposing it.
func TestNoCorrectionOverAPersonsDraft(t *testing.T) {
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
	if code := internalPost(t, e, "/v1/internal/discoveries/claim", nil, &job); code != 200 {
		t.Fatalf("claim: %d", code)
	}
	if code := internalPost(t, e, "/v1/internal/discoveries/"+itoa(d.ID)+"/result", map[string]any{"report": map[string]any{}, "document": job.Template}, nil); code != 204 {
		t.Fatalf("result: %d", code)
	}
	var vjob struct {
		ID       int64
		Document json.RawMessage
	}
	if code := internalPost(t, e, "/v1/internal/verifications/claim", nil, &vjob); code != 200 {
		t.Fatalf("verification claim: %d", code)
	}
	boss.must(200, "POST", "/v1/apps/blog/maps/1/submit", nil, nil) // the person takes it on
	var corrected map[string]any
	_ = json.Unmarshal(vjob.Document, &corrected)
	corrected["routes"] = corrected["routes"].([]any)[:1]
	score := map[string]any{"functions_total": 1, "functions_identical": 1, "inputs_total": 0, "inputs_protected": 0, "area_percent": 0}
	if code := internalPost(t, e, "/v1/internal/verifications/"+itoa(vjob.ID)+"/result", map[string]any{"score": score,
		"document": corrected, "before": score, "corrections": []map[string]any{{"entry": "x", "change": "narrowed", "detail": "d", "reason": "r"}}}, nil); code != 204 {
		t.Fatalf("verification result: %d", code)
	}
	var n int
	_ = e.st.DB.QueryRowContext(context.Background(), `SELECT count(*) FROM map_versions`).Scan(&n)
	var v1 struct{ Status string }
	boss.must(200, "GET", "/v1/apps/blog/maps/1", nil, &v1)
	var after struct {
		DraftVersion *int `json:"draft_version"`
	}
	boss.must(200, "GET", "/v1/apps/blog/discoveries/"+itoa(d.ID), nil, &after)
	if n != 1 || v1.Status != "in_review" || after.DraftVersion == nil || *after.DraftVersion != 1 {
		t.Fatalf("versions %d, v1 %q, discovery proposes %v", n, v1.Status, after.DraftVersion)
	}

}

// A person takes the draft on while the agent corrects it (after the
// agent's last check, before the supersede): the correction is withdrawn
// (retired, audited), the person's draft and the discovery's link stay. A
// trigger submits the draft the moment the corrected version is written.
func TestCorrectionWithdrawnWhenTakenOnMidway(t *testing.T) {
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
	if code := internalPost(t, e, "/v1/internal/discoveries/claim", nil, &job); code != 200 {
		t.Fatalf("claim: %d", code)
	}
	if code := internalPost(t, e, "/v1/internal/discoveries/"+itoa(d.ID)+"/result", map[string]any{"report": map[string]any{}, "document": job.Template}, nil); code != 204 {
		t.Fatalf("result: %d", code)
	}
	var vjob struct {
		ID       int64
		Document json.RawMessage
	}
	if code := internalPost(t, e, "/v1/internal/verifications/claim", nil, &vjob); code != 200 {
		t.Fatalf("verification claim: %d", code)
	}
	ctx := context.Background()
	for _, q := range []string{
		`CREATE FUNCTION take_on_midway() RETURNS trigger AS $$ BEGIN
			UPDATE map_versions SET status = 'in_review' WHERE app_id = NEW.app_id AND version = 1;
			RETURN NEW; END $$ LANGUAGE plpgsql`,
		`CREATE TRIGGER take_on_midway AFTER INSERT ON map_versions FOR EACH ROW WHEN (NEW.version = 2)
			EXECUTE FUNCTION take_on_midway()`,
	} {
		if _, err := e.st.DB.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	var corrected map[string]any
	_ = json.Unmarshal(vjob.Document, &corrected)
	corrected["routes"] = corrected["routes"].([]any)[:1]
	score := map[string]any{"functions_total": 1, "functions_identical": 1, "inputs_total": 0, "inputs_protected": 0, "area_percent": 0}
	if code := internalPost(t, e, "/v1/internal/verifications/"+itoa(vjob.ID)+"/result", map[string]any{"score": score,
		"document": corrected, "before": score, "corrections": []map[string]any{{"entry": "x", "change": "narrowed", "detail": "d", "reason": "r"}}}, nil); code != 204 {
		t.Fatalf("verification result: %d", code)
	}
	var v1, v2 struct {
		Status       string
		Verification struct {
			CorrectedVersion *int `json:"corrected_version"`
		}
	}
	boss.must(200, "GET", "/v1/apps/blog/maps/1", nil, &v1)
	boss.must(200, "GET", "/v1/apps/blog/maps/2", nil, &v2)
	var after struct {
		DraftVersion *int `json:"draft_version"`
	}
	boss.must(200, "GET", "/v1/apps/blog/discoveries/"+itoa(d.ID), nil, &after)
	var withdrawn, superseded int
	if err := e.st.DB.QueryRowContext(ctx, `SELECT count(*) FILTER (WHERE action = 'map.correction_withdrawn'),
		count(*) FILTER (WHERE action = 'map.superseded') FROM audit`).Scan(&withdrawn, &superseded); err != nil {
		t.Fatal(err)
	}
	if v1.Status != "in_review" || v1.Verification.CorrectedVersion != nil || v2.Status != "retired" ||
		after.DraftVersion == nil || *after.DraftVersion != 1 || withdrawn != 1 || superseded != 0 {
		t.Fatalf("v1 %+v, v2 %q, discovery proposes %v, withdrawn %d, superseded %d",
			v1, v2.Status, after.DraftVersion, withdrawn, superseded)
	}
}
