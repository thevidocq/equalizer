package api_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/exposure"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

func TestExposure(t *testing.T) {
	e := setup(t)
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host == "portal.acme.test" {
			_, _ = w.Write([]byte("<title>Portal</title>"))
			return
		}
		_, _ = w.Write([]byte("<title>Blog</title>"))
	}))
	defer legacy.Close()
	e.server.Exposure = &exposure.Runner{Store: e.st, Interval: time.Hour, Log: slog.New(slog.NewJSONHandler(io.Discard, nil)),
		Prober: &exposure.Prober{Timeout: time.Second, Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if addr == "8.8.8.8:80" {
				return (&net.Dialer{}).DialContext(ctx, network, legacy.Listener.Addr().String())
			}
			return nil, &net.OpError{Op: "dial", Err: net.UnknownNetworkError("refused")}
		}}}
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	boss := e.join("boss@acme.test")
	boss.must(201, "POST", "/v1/tenants/acme/apps", map[string]string{"id": "portal", "name": "Portal", "public_domain": "portal.acme.test"}, nil)

	boss.must(409, "POST", "/v1/apps/portal/exposure/check", nil, nil) // no IP yet
	for _, bad := range []string{"10.0.0.5", "127.0.0.1", "169.254.169.254", "portal.acme.test"} {
		boss.must(422, "PATCH", "/v1/apps/portal", map[string]any{"legacy_ip": bad}, nil)
	}
	var app store.App
	boss.must(200, "PATCH", "/v1/apps/portal", map[string]any{"legacy_ip": "8.8.8.8"}, &app)
	if app.Exposure.Status != "pending" {
		t.Fatalf("after setting the IP: %+v", app.Exposure)
	}
	boss.must(200, "POST", "/v1/apps/portal/exposure/check", nil, &app)
	if app.Exposure.Status != "exposed" || app.Exposure.Reason == nil || *app.Exposure.Reason != "answers_http" {
		t.Fatalf("exposed site: %+v", app.Exposure)
	}
	boss.must(429, "POST", "/v1/apps/portal/exposure/check", nil, nil)

	// The platform sees "exposed" and nothing more.
	var ov struct {
		Items []struct{ Sites []struct{ Status string } }
	}
	root.must(200, "GET", "/v1/platform/tenants", nil, &ov)
	if ov.Items[0].Sites[0].Status != "exposed" {
		t.Fatalf("platform overview: %+v", ov)
	}
	boss.must(200, "PATCH", "/v1/apps/portal", map[string]any{"legacy_ip": nil}, &app)
	if app.Exposure.Status != "not_configured" {
		t.Fatalf("after clearing the IP: %+v", app.Exposure)
	}

	// Equalizer's own address: the probe would reach Equalizer, not the site.
	e.server.LookupHost = func(_ context.Context, host string) ([]string, error) {
		if host == "panel.test" {
			return []string{"8.8.4.4"}, nil
		}
		return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	}
	boss.must(422, "PATCH", "/v1/apps/portal", map[string]any{"legacy_ip": "8.8.4.4"}, nil)
	boss.must(200, "PATCH", "/v1/apps/portal", map[string]any{"legacy_ip": "8.8.8.8:80"}, &app) // the port is dropped
	if app.Exposure.LegacyIP == nil || *app.Exposure.LegacyIP != "8.8.8.8" {
		t.Fatalf("with a port: %+v", app.Exposure)
	}
	// A site born behind Equalizer: nothing to probe, isolation fine.
	boss.must(200, "PATCH", "/v1/apps/portal", map[string]any{"never_public": true}, &app)
	if app.Exposure.Status != "never_public" || app.Exposure.LegacyIP != nil {
		t.Fatalf("never public: %+v", app.Exposure)
	}
	var st struct {
		Sites []struct {
			Checks []struct{ Name, State string }
		}
	}
	boss.must(200, "GET", "/v1/tenants/acme/status", nil, &st)
	for _, c := range st.Sites[0].Checks {
		if c.Name == "Isolation" && c.State != "ok" {
			t.Fatalf("isolation of a never public site: %+v", c)
		}
	}
	boss.must(200, "PATCH", "/v1/apps/portal", map[string]any{"never_public": false}, &app)
	if app.Exposure.Status != "not_configured" {
		t.Fatalf("back: %+v", app.Exposure)
	}
}
