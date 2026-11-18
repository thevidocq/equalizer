package limits

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
)

func testModel() *mapmodel.Model {
	return &mapmodel.Model{
		Meta:     mapmodel.Meta{Tenant: "t", App: "a"},
		Defaults: mapmodel.Defaults{RateLimit: mapmodel.RateLimit{PerIP: "5/min"}, LegacyBudget: mapmodel.LegacyBudget{MaxRPS: 2.5}},
		Routes:   []mapmodel.Route{{ID: "detail"}},
		Forms:    []mapmodel.Form{{ID: "login", RateLimit: mapmodel.RateLimit{PerIP: "2/min"}}},
		Behavior: []mapmodel.BehaviorRule{
			{ID: "enum", Route: "detail", Signal: "distinct_param_values", Param: "id",
				Threshold: mapmodel.Threshold{Count: 3, Window: "10m"}, Action: []string{"throttle", "alert"}},
			{ID: "brute", Route: "login", Signal: "failed_logins",
				Threshold: mapmodel.Threshold{Count: 2, Window: "15m"}, Action: []string{"block", "alert"}},
			{ID: "later", Signal: "missing_predecessor", Threshold: mapmodel.Threshold{Count: 1, Window: "1m"}, Action: []string{"alert"}},
		},
	}
}

func run(t *testing.T, store Store) {
	ctx := context.Background()
	s, notes, err := Compile(testModel(), store)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 {
		t.Errorf("notes: %v", notes)
	}
	ip1 := Subject{IP: "198.51.100.1"}
	ip2 := Subject{IP: "198.51.100.2"}

	// Default limit (5/min per IP), separate per IP.
	for i := 1; i <= 6; i++ {
		d, err := s.Check(ctx, ip1, "detail", true)
		if err != nil {
			t.Fatal(err)
		}
		if (i <= 5) != d.Allowed() {
			t.Fatalf("request %d: %+v", i, d)
		}
		if i == 6 && (d.Status != 429 || d.Reason != "per_ip" || d.RetryAfter <= 0 || d.RetryAfter > time.Minute) {
			t.Fatalf("over the limit: %+v", d)
		}
	}
	if d, _ := s.Check(ctx, ip2, "detail", true); !d.Allowed() {
		t.Fatal("another IP was limited")
	}

	// Form limit on top of the default one.
	for i := 1; i <= 3; i++ {
		d, _ := s.Check(ctx, ip2, "login", true)
		if (i <= 2) != d.Allowed() {
			t.Fatalf("login %d: %+v", i, d)
		}
	}

	// Enumeration: the third distinct id crosses the threshold.
	ip3 := Subject{IP: "203.0.113.3"}
	for i, id := range []string{"1", "2", "2", "3"} {
		alerts, d, err := s.Observe(ctx, ip3, "detail", Observation{Params: map[string]string{"id": id}})
		if err != nil {
			t.Fatal(err)
		}
		if i < 3 && (!d.Allowed() || len(alerts) != 0) {
			t.Fatalf("observation %d: %+v %v", i, d, alerts)
		}
		if i == 3 && (d.Status != 429 || len(alerts) != 1 || alerts[0].Rule != "enum") {
			t.Fatalf("threshold: %+v %v", d, alerts)
		}
	}
	if d, _ := s.Check(ctx, ip3, "detail", false); d.Status != 429 || d.Reason != "behavior_throttle" {
		t.Fatalf("throttle not applied: %+v", d)
	}
	if d, _ := s.Check(ctx, ip3, "login", false); !d.Allowed() {
		t.Fatalf("a throttle applies only to its route: %+v", d)
	}

	// Brute force: a block applies to the whole site.
	ip4 := Subject{IP: "203.0.113.4"}
	_, d, _ := s.Observe(ctx, ip4, "login", Observation{FailedLogin: false})
	_, d, _ = s.Observe(ctx, ip4, "login", Observation{FailedLogin: true})
	if !d.Allowed() {
		t.Fatal("blocked too early")
	}
	alerts, d, _ := s.Observe(ctx, ip4, "login", Observation{FailedLogin: true})
	if d.Status != 403 || len(alerts) != 1 {
		t.Fatalf("brute force: %+v %v", d, alerts)
	}
	if d, _ := s.Check(ctx, ip4, "detail", false); d.Status != 403 || d.Rule != "brute" {
		t.Fatalf("block not site-wide: %+v", d)
	}

	// Legacy budget: ceil(2.5) = 3 per second across replicas.
	allowed := 0
	for i := 0; i < 5; i++ {
		if ok, _ := s.LegacyAllowed(ctx); ok {
			allowed++
		}
	}
	if allowed < 3 || allowed > 4 { // the second may roll over during the loop
		t.Errorf("legacy rps: %d allowed", allowed)
	}
}

func TestMemory(t *testing.T) { run(t, NewMemory()) }

func TestRedis(t *testing.T) {
	bin, err := exec.LookPath("redis-server")
	if err != nil {
		t.Skip("redis-server not installed")
	}
	port := 16392
	cmd := exec.Command(bin, "--port", strconv.Itoa(port), "--save", "", "--appendonly", "no")
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Start(); err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	c := redis.NewClient(&redis.Options{Addr: "127.0.0.1:" + strconv.Itoa(port)})
	defer c.Close()
	for i := 0; c.Ping(context.Background()).Err() != nil; i++ {
		if i > 50 {
			t.Fatal("redis did not start")
		}
		time.Sleep(50 * time.Millisecond)
	}
	run(t, NewRedis(c))
}

func TestParseRate(t *testing.T) {
	for v, want := range map[string]time.Duration{"60/min": time.Minute, "5/hour": time.Hour, "1/s": time.Second, "3/day": 24 * time.Hour} {
		if _, w, err := ParseRate(v); err != nil || w != want {
			t.Errorf("%s: %v %v", v, w, err)
		}
	}
	for _, bad := range []string{"0/min", "x/min", "5/week", "5"} {
		if _, _, err := ParseRate(bad); err == nil {
			t.Errorf("%s accepted", bad)
		}
	}
}
