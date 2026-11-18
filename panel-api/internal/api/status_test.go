package api_test

import "testing"

func TestStatus(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	boss := e.join("boss@acme.test")
	boss.must(201, "POST", "/v1/tenants/acme/apps", map[string]string{"id": "portal", "name": "Portal", "public_domain": "portal.acme.test"}, nil)

	var st struct {
		Sites []struct {
			State  string
			Checks []struct{ Name, State string }
			Days   []struct{ Date string }
		}
		LogsAvailable bool `json:"logs_available"`
	}
	boss.must(200, "GET", "/v1/tenants/acme/status", nil, &st)
	if len(st.Sites) != 1 || st.Sites[0].State != "down" || st.Sites[0].Checks[0].Name != "Map" || len(st.Sites[0].Days) != 7 || st.LogsAvailable {
		t.Fatalf("status: %+v", st)
	}
	root.must(403, "GET", "/v1/tenants/acme/status", nil, nil) // platform users do not see tenants' sites
	boss.must(403, "GET", "/v1/status/platform", nil, nil)
	var pf struct {
		Services []struct{ Name, State string }
	}
	root.must(200, "GET", "/v1/status/platform", nil, &pf)
	if len(pf.Services) == 0 || pf.Services[0].Name != "Database" || pf.Services[0].State != "ok" {
		t.Fatalf("platform: %+v", pf)
	}
}
