package server

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/clientip"
	"github.com/thevidocq/equalizer/gateway/internal/formtoken"
	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/session"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
)

func rebuild(t *testing.T, st *site.Site, change func(m *mapmodel.Model)) *site.Site {
	t.Helper()
	change(st.Model)
	out, err := site.Build(st.Model)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestRateLimits(t *testing.T) {
	calls := 0
	st := rebuild(t, testSite(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "text/css")
		_, _ = w.Write([]byte("x"))
	})), func(m *mapmodel.Model) {
		m.Defaults.RateLimit = mapmodel.RateLimit{PerIP: "3/min"}
	})
	var events bytes.Buffer
	h := newPublic(t, st, &events)

	for i := 1; i <= 4; i++ {
		rec := get(h, "GET", "/", nil)
		if i <= 3 && rec.Code == http.StatusTooManyRequests {
			t.Fatalf("request %d limited too early", i)
		}
		if i == 4 {
			if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
				t.Fatalf("over the limit: %d %v", rec.Code, rec.Header())
			}
			if ev := lastEvent(t, &events); ev["type"] != "ratelimit.hit" || ev["reason"] != "per_ip" {
				t.Errorf("event: %v", ev)
			}
		}
	}
	before := calls
	// Assets are not counted against the page limits.
	for i := 0; i < 5; i++ {
		if rec := get(h, "GET", "/css/site.css", nil); rec.Code != 200 {
			t.Fatalf("asset %d: %d", i, rec.Code)
		}
	}
	if calls != before+5 {
		t.Errorf("assets reached the legacy app %d times", calls-before)
	}
}

func TestEnumerationThrottled(t *testing.T) {
	calls := 0
	st := rebuild(t, testSite(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ })), func(m *mapmodel.Model) {
		m.Behavior = []mapmodel.BehaviorRule{{ID: "permit_enumeration", Route: "detail", Signal: "distinct_param_values", Param: "id",
			Threshold: mapmodel.Threshold{Count: 3, Window: "10m"}, Action: []string{"throttle", "alert"}}}
	})
	var events bytes.Buffer
	h := newPublic(t, st, &events)
	for _, id := range []string{"1", "2", "2"} {
		if rec := get(h, "GET", "/permits/"+id+"?tab=data", nil); rec.Code != 200 {
			t.Fatalf("permit %s: %d", id, rec.Code)
		}
	}
	if rec := get(h, "GET", "/permits/3?tab=data", nil); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("third distinct id: %d", rec.Code)
	}
	if !strings.Contains(events.String(), `"type":"behavior.alert"`) || !strings.Contains(events.String(), `"reason":"permit_enumeration"`) {
		t.Errorf("alert not emitted: %s", events.String())
	}
	if calls != 3 {
		t.Errorf("legacy called %d times, want 3 (the throttled request never reaches it)", calls)
	}
	if rec := get(h, "GET", "/", nil); rec.Code != 200 {
		t.Errorf("a throttle applies only to its route: %d", rec.Code)
	}
}

func TestBruteForceBlocked(t *testing.T) {
	legacy := &loginLegacy{sessions: map[string]bool{}}
	st := rebuild(t, sessionSite(t, legacy), func(m *mapmodel.Model) {
		m.Behavior = []mapmodel.BehaviorRule{{ID: "login_bruteforce", Route: "login", Signal: "failed_logins",
			Threshold: mapmodel.Threshold{Count: 3, Window: "15m"}, Action: []string{"block", "alert"}}}
	})
	sealer, _ := formtoken.New(formtoken.RandomKey())
	mgr, _ := session.NewManager(session.NewMemory(), formtoken.RandomKey())
	var events bytes.Buffer
	p := &Public{Maps: fixedMaps{st}, IPs: clientip.New(nil), Forms: sealer, Sessions: mgr,
		Log: slog.New(slog.NewJSONHandler(io.Discard, nil)), Events: telemetry.NewEmitter(&events, "test")}
	b := &browser{t: t, h: p.Handler(), cookies: map[string]string{}}

	token := tokenRe.FindStringSubmatch(b.do("GET", "/login", nil).Body.String())[1]
	for i := 1; i <= 3; i++ {
		rec := b.do("POST", "/login", url.Values{"username": {"mary"}, "password": {"guess" + string(rune('0'+i))}, "_eq_token": {token}})
		if rec.Code != 200 {
			t.Fatalf("failed login %d: %d", i, rec.Code)
		}
	}
	// Blocked everywhere, even with the right password.
	rec := b.do("POST", "/login", url.Values{"username": {"mary"}, "password": {"s3cret'pw"}, "_eq_token": {token}})
	if rec.Code != http.StatusForbidden || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("after brute force: %d %v", rec.Code, rec.Header())
	}
	if rec := b.do("GET", "/login", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("block is not site-wide: %d", rec.Code)
	}
	if strings.Count(events.String(), `"type":"behavior.alert"`) != 1 {
		t.Errorf("expected one alert: %s", events.String())
	}
	if strings.Contains(events.String(), "guess") {
		t.Fatal("passwords in the events")
	}
}
