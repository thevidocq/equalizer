package api_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/thevidocq/equalizer/panel-api/internal/api"
)

// A map may use only its own tenant's connectors, and direct mode only
// towards destinations the platform allows.
func TestMapReach(t *testing.T) {
	e := setup(t)
	e.server.DirectDestinations, _ = api.ParseDestinations("vuln-app:80")
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	for _, id := range []string{"acme", "other"} {
		root.must(201, "POST", "/v1/tenants", map[string]string{"id": id, "name": id, "admin_email": "boss@" + id + ".test", "admin_name": "Boss"}, nil)
	}
	acme, other := e.join("boss@acme.test"), e.join("boss@other.test")
	acme.must(201, "POST", "/v1/tenants/acme/connectors", map[string]string{"id": "cn-1"}, nil)
	acme.must(201, "POST", "/v1/tenants/acme/apps", map[string]string{"id": "portal", "name": "Portal", "public_domain": "portal.acme.test"}, nil)
	acme.must(201, "POST", "/v1/tenants/acme/apps", map[string]string{"id": "taxes", "name": "Taxes", "public_domain": "taxes.acme.test"}, nil)
	other.must(201, "POST", "/v1/tenants/other/apps", map[string]string{"id": "intranet", "name": "Intranet", "public_domain": "intranet.other.test"}, nil)

	draft := func(c *client, app, doc string) (int, map[string]any) {
		var out map[string]any
		return c.do("POST", "/v1/apps/"+app+"/maps", map[string]any{"document": json.RawMessage(doc)}, &out), out
	}
	// Another tenant's connector: refused, and indistinguishable from a missing one.
	if code, out := draft(other, "intranet", mapDoc); code != 422 || out["error"] != "connector_not_found" {
		t.Fatalf("map with another tenant's connector: %d %v", code, out)
	}
	if code, out := draft(acme, "portal", strings.Replace(mapDoc, `"cn-1"`, `"cn-9"`, 1)); code != 422 || out["error"] != "connector_not_found" {
		t.Fatalf("map with an unknown connector: %d %v", code, out)
	}
	// Direct mode towards the platform's own services.
	direct := strings.Replace(mapDoc, `"mode":"connector","connector_id":"cn-1","allow":["10.0.0.1:8080"]`, `"mode":"direct","allow":["redis:6379"]`, 1)
	if code, out := draft(acme, "portal", direct); code != 422 || out["error"] != "direct_not_allowed" {
		t.Fatalf("direct map towards redis: %d %v", code, out)
	}
	labDirect := strings.Replace(mapDoc, `"mode":"connector","connector_id":"cn-1","allow":["10.0.0.1:8080"]`, `"mode":"direct","allow":["vuln-app:80"]`, 1)
	if code, out := draft(acme, "portal", labDirect); code != 201 {
		t.Fatalf("direct map towards an allowed destination: %d %v", code, out)
	}
	// One connector, two sites on the same server (virtual hosts).
	vhost := strings.Replace(mapDoc, `"base_url":"http://10.0.0.1:8080","reach":{"mode":"connector","connector_id":"cn-1","allow":["10.0.0.1:8080"]}`,
		`"base_url":"https://taxes.acme.test","reach":{"mode":"connector","connector_id":"cn-1","address":"10.0.0.1:443","allow":["10.0.0.1:443"]}`, 1)
	if code, out := draft(acme, "taxes", vhost); code != 201 {
		t.Fatalf("virtual-host map: %d %v", code, out)
	}
	badAddr := strings.Replace(vhost, `"address":"10.0.0.1:443"`, `"address":"10.0.0.2:443"`, 1)
	if code, _ := draft(acme, "taxes", badAddr); code != 422 {
		t.Fatalf("address outside allow: %d", code)
	}

	// Publish both portal (v2, connector) and taxes with an approver.
	acme.must(201, "POST", "/v1/tenants/acme/invitations", map[string]string{"email": "ap@acme.test", "name": "Ap", "role": "approver"}, nil)
	ap := e.join("ap@acme.test")
	acme.must(201, "POST", "/v1/apps/portal/maps", map[string]any{"document": json.RawMessage(mapDoc)}, nil)
	for _, v := range []string{"/v1/apps/portal/maps/2", "/v1/apps/taxes/maps/1"} {
		acme.must(200, "POST", v+"/submit", nil, nil)
		ap.must(200, "POST", v+"/approve", nil, nil)
		ap.must(200, "POST", v+"/publish", nil, nil)
	}
	var conns struct {
		Items []struct {
			ID     string
			UsedBy []string `json:"used_by"`
		}
	}
	acme.must(200, "GET", "/v1/tenants/acme/connectors", nil, &conns)
	if len(conns.Items) != 1 || strings.Join(conns.Items[0].UsedBy, ",") != "portal,taxes" {
		t.Fatalf("connector used by: %+v", conns.Items)
	}
	var out map[string]any
	if code := acme.do("DELETE", "/v1/tenants/acme/connectors/cn-1", nil, &out); code != 409 || out["error"] != "connector_in_use" {
		t.Fatalf("deleting a connector in use: %d %v", code, out)
	}
	other.must(404, "DELETE", "/v1/tenants/other/connectors/cn-1", nil, nil)

	// The gateway gets every published map, each with its domain.
	var maps struct {
		Items []struct{ Payload string }
	}
	_ = json.Unmarshal(internalGet(t, e, "/v1/internal/maps", internalToken), &maps)
	if len(maps.Items) != 2 {
		t.Fatalf("internal maps: %+v", maps)
	}
	if code := internalStatus(t, e, "/v1/internal/maps", "wrong"); code != 401 {
		t.Fatalf("internal maps without token: %d", code)
	}
}

func TestTemplates(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	boss := e.join("boss@acme.test")
	boss.must(422, "POST", "/v1/tenants/acme/apps", map[string]any{"id": "blog", "name": "Blog", "public_domain": "blog.acme.test", "kind": "typo3"}, nil)
	var app struct{ Kind *string }
	boss.must(201, "POST", "/v1/tenants/acme/apps", map[string]any{"id": "blog", "name": "Blog", "public_domain": "blog.acme.test", "kind": "wordpress"}, &app)
	if app.Kind == nil || *app.Kind != "wordpress" {
		t.Fatalf("kind: %+v", app)
	}
	var tl struct{ Items []struct{ ID string } }
	boss.must(200, "GET", "/v1/map-templates", nil, &tl)
	if len(tl.Items) != 5 {
		t.Fatalf("templates: %+v", tl)
	}
	boss.must(201, "POST", "/v1/tenants/acme/connectors", map[string]string{"id": "cn-web01"}, nil)
	var out struct{ Document json.RawMessage }
	boss.must(200, "POST", "/v1/apps/blog/map-template", map[string]string{"template": "wordpress", "connector_id": "cn-web01", "address": "127.0.0.1:8443"}, &out)
	if !strings.Contains(string(out.Document), `"https://blog.acme.test"`) || !strings.Contains(string(out.Document), `"wp-admin"`) {
		t.Fatalf("rendered template: %s", out.Document)
	}
	// The filled template is a valid draft.
	boss.must(201, "POST", "/v1/apps/blog/maps", map[string]any{"document": out.Document}, nil)
	boss.must(404, "POST", "/v1/apps/blog/map-template", map[string]string{"template": "nope", "connector_id": "cn-web01", "address": "127.0.0.1:8443"}, nil)
	boss.must(422, "POST", "/v1/apps/blog/map-template", map[string]string{"template": "wordpress", "connector_id": "cn-web01", "address": "no-port"}, nil)
}
