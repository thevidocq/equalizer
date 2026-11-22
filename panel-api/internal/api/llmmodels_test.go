package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thevidocq/equalizer/panel-api/internal/llmmodels"
)

func TestLLMModels(t *testing.T) {
	e := setup(t)
	var gotKey string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("x-api-key")
		if gotKey != "sk-good" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"claude-haiku-5-5","display_name":"Claude Haiku 5.5"}]}`))
	}))
	defer provider.Close()
	e.server.Models = llmmodels.Lister{Client: provider.Client(), AnthropicURL: provider.URL}
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	boss := e.join("boss@acme.test")

	path := "/v1/tenants/acme/llm/models"
	boss.must(422, "POST", path, map[string]string{"provider": "anthropic"}, nil) // no key yet
	boss.must(422, "POST", path, map[string]string{"provider": "anthropic", "api_key": "sk-bad"}, nil)
	var list struct{ Items []struct{ ID, Name string } }
	boss.must(200, "POST", path, map[string]string{"provider": "anthropic", "api_key": "sk-good"}, &list)
	if len(list.Items) != 1 || list.Items[0].Name != "Claude Haiku 5.5" {
		t.Fatalf("models: %+v", list)
	}
	// Once saved, the key is used again without being typed (it is never returned).
	boss.must(200, "PUT", "/v1/tenants/acme/llm", map[string]any{"provider": "anthropic", "model": "claude-haiku-5-5", "api_key": "sk-good"}, nil)
	gotKey = ""
	boss.must(200, "POST", path, map[string]string{"provider": "anthropic"}, &list)
	if gotKey != "sk-good" {
		t.Fatalf("saved key not used: %q", gotKey)
	}
	root.must(403, "POST", path, map[string]string{"provider": "anthropic", "api_key": "sk-good"}, nil)
}

func TestAlertKindsAPI(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	boss := e.join("boss@acme.test")
	var a struct{ Kinds map[string]bool }
	boss.must(200, "GET", "/v1/tenants/acme/alerts", nil, &a)
	if len(a.Kinds) != 7 || !a.Kinds["recovered"] || !a.Kinds["draft_ready"] {
		t.Fatalf("defaults: %+v", a)
	}
	boss.must(200, "PUT", "/v1/tenants/acme/alerts", map[string]any{"enabled": true, "extra_emails": []string{}, "kinds": map[string]bool{"recovered": false}}, &a)
	boss.must(200, "PUT", "/v1/tenants/acme/alerts", map[string]any{"enabled": true, "extra_emails": []string{}, "kinds": map[string]bool{"connector": false}}, &a)
	if a.Kinds["recovered"] || a.Kinds["connector"] || !a.Kinds["isolation"] {
		t.Fatalf("kinds kept and changed: %+v", a.Kinds)
	}
	boss.must(422, "PUT", "/v1/tenants/acme/alerts", map[string]any{"enabled": true, "extra_emails": []string{}, "kinds": map[string]bool{"nope": true}}, nil)
}

func TestAbout(t *testing.T) {
	e := setup(t)
	e.server.Version = "9.9.9"
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	boss := e.join("boss@acme.test")
	var a struct {
		Release  string
		Services []struct{ Name string }
	}
	boss.must(200, "GET", "/v1/about", nil, &a)
	if a.Release != "9.9.9" || a.Services != nil {
		t.Fatalf("tenant view: %+v", a)
	}
	e.client().must(401, "GET", "/v1/about", nil, nil) // not before logging in
}

func TestTimeZone(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	var me struct {
		TimeZone string `json:"time_zone"`
	}
	root.must(200, "GET", "/v1/auth/me", nil, &me)
	if me.TimeZone != "UTC" {
		t.Fatalf("default: %q", me.TimeZone)
	}
	root.must(422, "PATCH", "/v1/auth/me", map[string]string{"time_zone": "Mars/Olympus"}, nil)
	root.must(422, "PATCH", "/v1/auth/me", map[string]string{"time_zone": "Local"}, nil)
	root.must(200, "PATCH", "/v1/auth/me", map[string]string{"time_zone": "Europe/Rome"}, nil)
	root.must(200, "GET", "/v1/auth/me", nil, &me)
	if me.TimeZone != "Europe/Rome" {
		t.Fatalf("saved: %q", me.TimeZone)
	}
}
