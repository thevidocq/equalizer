package api_test

import (
	"encoding/json"
	"testing"
)

// What visitors asked for is added by a person in one click, from the
// discovery's report: the request names the item, the change comes from the
// report, and lands in a new draft.
func TestAdoptFromTheReport(t *testing.T) {
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
	report := map[string]any{
		"files_refused": []map[string]any{
			{"path": "/wp-content/uploads/price-list.csv", "clients": 3, "verdict": "site", "why": "linked",
				"add": map[string]any{"prefix": "/wp-content/uploads/", "type": "text/csv"}},
			{"path": "/wp-content/uploads/x.bin", "clients": 1, "verdict": "suspect", "why": "unknown", "add": nil}},
		"query_dropped": []map[string]any{
			{"route": "/{slug}/", "route_id": "page", "key": "orderby", "clients": 2, "verdict": "site", "why": "linked",
				"add": map[string]any{"route_id": "page", "key": "orderby"}}},
	}
	if code := internalPost(t, e, "/v1/internal/discoveries/"+itoa(d.ID)+"/result", map[string]any{"report": report, "document": job.Template}, nil); code != 204 {
		t.Fatalf("result: %d", code)
	}
	adopt := "/v1/apps/blog/discoveries/" + itoa(d.ID) + "/adopt"

	var v struct{ Version int }
	boss.must(201, "POST", adopt, map[string]any{"kind": "file", "path": "/wp-content/uploads/price-list.csv"}, &v)
	var m struct {
		Note     string
		Document struct {
			Assets []struct {
				Prefix string
				Types  []string
			}
			Routes []struct {
				ID            string
				Query         map[string]map[string]any
				LegacyRequest struct {
					Query map[string]string
				} `json:"legacy_request"`
			}
		}
	}
	boss.must(200, "GET", "/v1/apps/blog/maps/"+itoa(int64(v.Version)), nil, &m)
	uploads := false
	for _, a := range m.Document.Assets {
		if a.Prefix == "/wp-content/uploads/" {
			for _, ty := range a.Types {
				uploads = uploads || ty == "text/csv"
			}
		}
	}
	if v.Version != 2 || !uploads {
		t.Fatalf("file adopted: version %d, %+v", v.Version, m.Document.Assets)
	}

	// On top of the discovery's draft, still a draft: the key goes on version 1 again.
	boss.must(201, "POST", adopt, map[string]any{"kind": "key", "route": "/{slug}/", "key": "orderby"}, &v)
	boss.must(200, "GET", "/v1/apps/blog/maps/"+itoa(int64(v.Version)), nil, &m)
	found := false
	for _, r := range m.Document.Routes {
		if r.ID == "page" {
			spec, _ := json.Marshal(r.Query["orderby"])
			found = r.LegacyRequest.Query["orderby"] == "{{query.orderby}}" && string(spec) != "null"
		}
	}
	if !found {
		t.Fatalf("key adopted: %+v", m.Document.Routes)
	}

	// Only what the report proposes, never what the request says.
	boss.must(422, "POST", adopt, map[string]any{"kind": "file", "path": "/wp-content/uploads/x.bin"}, nil)
	boss.must(422, "POST", adopt, map[string]any{"kind": "file", "path": "/wp-content/uploads/other.sql"}, nil)
	boss.must(422, "POST", adopt, map[string]any{"kind": "key", "route": "/{slug}/", "key": "redirect_to"}, nil)
	boss.must(400, "POST", adopt, map[string]any{"kind": "file", "path": "/wp-content/uploads/price-list.csv", "type": "text/html"}, nil) // no type from the request
}
