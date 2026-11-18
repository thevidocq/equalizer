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
}
