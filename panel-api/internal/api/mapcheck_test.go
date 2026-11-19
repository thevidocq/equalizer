package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/thevidocq/equalizer/panel-api/internal/mapcheck"
)

func TestMapCheckedByGateway(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	admin := e.join("boss@acme.test")
	admin.must(201, "POST", "/v1/tenants/acme/apps", map[string]string{"id": "portal", "name": "Portal", "public_domain": "portal.acme.test"}, nil)
	admin.must(201, "POST", "/v1/tenants/acme/connectors", map[string]string{"id": "cn-1"}, nil)
	admin.must(201, "POST", "/v1/tenants/acme/invitations", map[string]string{"email": "ap@acme.test", "name": "Ap", "role": "approver"}, nil)
	ap := e.join("ap@acme.test")

	var mode atomic.Value // ok, refuse, down
	mode.Store("ok")
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch mode.Load() {
		case "refuse":
			w.WriteHeader(422)
			_, _ = w.Write([]byte(`{"ok":false,"error":"route \"home\": role \"staff\" is not in roles.list"}`))
		case "down":
			w.WriteHeader(503)
		default:
			_, _ = w.Write([]byte(`{"ok":true}`))
		}
	}))
	defer gw.Close()
	e.server.MapCheck = &mapcheck.Checker{Addr: strings.TrimPrefix(gw.URL, "http://")}
	doc := map[string]any{"document": json.RawMessage(mapDoc)}

	mode.Store("refuse")
	var bad struct{ Error, Message string }
	if code := admin.do("POST", "/v1/apps/portal/maps", doc, &bad); code != 422 || bad.Error != "map_inconsistent" || !strings.Contains(bad.Message, "staff") {
		t.Fatalf("inconsistent draft: %d %+v", code, bad)
	}
	mode.Store("down")
	admin.must(201, "POST", "/v1/apps/portal/maps", doc, nil) // a draft is not live
	admin.must(200, "POST", "/v1/apps/portal/maps/1/submit", nil, nil)
	ap.must(200, "POST", "/v1/apps/portal/maps/1/approve", nil, nil)
	ap.must(503, "POST", "/v1/apps/portal/maps/1/publish", nil, nil) // never published unchecked
	mode.Store("refuse")
	ap.must(422, "POST", "/v1/apps/portal/maps/1/publish", nil, nil)
	mode.Store("ok")
	ap.must(200, "POST", "/v1/apps/portal/maps/1/publish", nil, nil)
}
