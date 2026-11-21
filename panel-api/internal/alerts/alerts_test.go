package alerts

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/health"
	"github.com/thevidocq/equalizer/panel-api/internal/mail"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

type fake struct {
	settings store.AlertSettings
	known    map[string]store.SiteAlert
}

func (f *fake) Tenants(context.Context, *string) ([]store.Tenant, error) {
	return []store.Tenant{{ID: "acme", Name: "Acme"}}, nil
}
func (f *fake) Users(context.Context, string) ([]auth.User, error) {
	admin, viewer := "admin", "viewer"
	return []auth.User{{Email: "alex@acme.example", TenantRole: &admin}, {Email: "sam@acme.example", TenantRole: &viewer}}, nil
}
func (f *fake) Alerts(context.Context, string) (store.AlertSettings, error) { return f.settings, nil }
func (f *fake) SiteAlerts(context.Context, string) (map[string]store.SiteAlert, error) {
	out := map[string]store.SiteAlert{}
	for k, v := range f.known {
		out[k] = v
	}
	return out, nil
}
func (f *fake) SaveSiteAlert(_ context.Context, _, check string, a store.SiteAlert) error {
	f.known[check] = a
	return nil
}
func (f *fake) Record(context.Context, *int64, *string, string, string, map[string]any) error {
	return nil
}

func TestAlerts(t *testing.T) {
	f := &fake{settings: store.AlertSettings{Enabled: true, ExtraEmails: []string{"oncall@acme.example"}}, known: map[string]store.SiteAlert{}}
	state := health.OK
	now := time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)
	rec := &mail.Recorder{}
	r := &Runner{Store: f, Mail: rec, PanelURL: "https://panel.test", Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now: func() time.Time { return now },
		Statuses: func(context.Context, string) ([]health.Site, bool, error) {
			return []health.Site{{App: "blog", Name: "Company blog", PublicDomain: "blog.acme.example", Checks: []health.Check{
				{Name: "Map", State: health.Down, Detail: "no published map"},
				{Name: "Connector", State: state, Detail: "cn-web-01"},
			}}}, true, nil
		}}
	step := func(d time.Duration) int { now = now.Add(d); r.Once(context.Background()); return len(rec.Messages()) }

	if n := step(0); n != 0 {
		t.Fatalf("working site, or a missing map, sent %d emails", n)
	}
	state = health.Down
	if n := step(time.Minute); n != 0 {
		t.Fatal("alert before the grace period")
	}
	if n := step(4 * time.Minute); n != 2 {
		t.Fatalf("problem: %d emails, want 2 (admin and extra address; not the viewer)", n)
	}
	m := rec.Messages()[0]
	if !strings.Contains(m.Subject, "1 site problem") || !strings.Contains(m.Body, "PROBLEM  Company blog") || !strings.Contains(m.Body, "/t/acme/status") {
		t.Errorf("email: %q %q", m.Subject, m.Body)
	}
	if n := step(10 * time.Minute); n != 2 {
		t.Fatal("the same problem sent twice")
	}
	state = health.OK
	if n := step(time.Minute); n != 4 || !strings.Contains(rec.Messages()[3].Body, "WORKING  Company blog") {
		t.Fatalf("recovery: %d emails", n)
	}
	// A short blip never reaches anyone.
	state = health.Down
	step(time.Minute)
	state = health.OK
	if n := step(time.Minute); n != 4 {
		t.Fatalf("blip sent: %d", n)
	}
	f.settings.Enabled = false
	state = health.Down
	step(time.Minute)
	if n := step(10 * time.Minute); n != 4 {
		t.Fatal("alerts sent while off")
	}
}

// Each kind of alert is turned on or off on its own; the state is followed
// all the same, so turning a kind back on does not replay old problems.
func TestAlertKinds(t *testing.T) {
	f := &fake{settings: store.AlertSettings{Enabled: true, Kinds: map[string]bool{"connector": false, "recovered": false}},
		known: map[string]store.SiteAlert{}}
	conn, legacy := health.OK, health.OK
	now := time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)
	rec := &mail.Recorder{}
	r := &Runner{Store: f, Mail: rec, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Now: func() time.Time { return now },
		Statuses: func(context.Context, string) ([]health.Site, bool, error) {
			return []health.Site{{App: "blog", Name: "Blog", Checks: []health.Check{
				{Name: "Connector", State: conn}, {Name: "Legacy site", State: legacy},
			}}}, true, nil
		}}
	step := func(d time.Duration) int { now = now.Add(d); r.Once(context.Background()); return len(rec.Messages()) }
	step(0)
	conn = health.Down
	step(time.Minute)
	if n := step(5 * time.Minute); n != 0 {
		t.Fatalf("muted connector alert sent: %d", n)
	}
	legacy = health.Down
	step(time.Minute)
	if n := step(5 * time.Minute); n != 1 || !strings.Contains(rec.Messages()[0].Body, "legacy site") || strings.Contains(rec.Messages()[0].Body, "connector") {
		t.Fatalf("legacy alert: %d %v", n, rec.Messages())
	}
	conn, legacy = health.OK, health.OK
	if n := step(time.Minute); n != 1 {
		t.Fatalf("muted recovery sent: %d", n)
	}
}

func TestWhen(t *testing.T) {
	at := time.Date(2026, 11, 21, 1, 33, 7, 0, time.UTC)
	rome, _ := time.LoadLocation("Europe/Rome")
	kolkata, _ := time.LoadLocation("Asia/Kolkata")
	for _, c := range []struct {
		loc  *time.Location
		want string
	}{
		{time.UTC, "21 Nov 2026, 01:33:07 UTC"},
		{rome, "21 Nov 2026, 02:33:07 UTC+1"},
		{kolkata, "21 Nov 2026, 07:03:07 UTC+5:30"},
	} {
		if got := When(at.In(c.loc)); got != c.want {
			t.Errorf("%s: %q, want %q", c.loc, got, c.want)
		}
	}
}
