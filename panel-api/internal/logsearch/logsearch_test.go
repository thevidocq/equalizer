package logsearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestQueries(t *testing.T) {
	f := Filter{Tenant: "acme", App: "portal", Types: []string{"request.blocked", "field.rejected"}, Search: `x" | tenant="other`}
	if err := f.Check(); err != nil {
		t.Fatal(err)
	}
	q := f.Logs()
	want := `{stream="security"} |= "x\" | tenant=\"other" | json | tenant="acme" | app="portal" | type=~"^(request\\.blocked|field\\.rejected)$"`
	if q != want {
		t.Fatalf("logs query:\n%s\nwant\n%s", q, want)
	}
	if !strings.HasPrefix(f.Counts(), `sum by (type) (count_over_time({stream="security"}`) || !strings.HasSuffix(f.Counts(), `[24h]))`) {
		t.Errorf("counts: %s", f.Counts())
	}
	if !strings.Contains(f.Refused(), `type!="request.served" [3600s]`) {
		t.Errorf("refused: %s", f.Refused())
	}
	for name, bad := range map[string]Filter{
		"type":   {Tenant: "acme", Types: []string{"anything"}},
		"since":  {Tenant: "acme", Since: "1y"},
		"app":    {Tenant: "acme", App: `a"b`},
		"tenant": {Tenant: `acme"}`},
		"search": {Tenant: "acme", Search: "a\nb"},
	} {
		if bad.Check() == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestEvents(t *testing.T) {
	now := time.Date(2026, 11, 18, 22, 0, 0, 0, time.UTC)
	line := func(ts time.Time, tenant, typ string) string {
		b, _ := json.Marshal(map[string]any{"ts": ts, "type": typ, "tenant": tenant, "app": "portal", "path": "/x",
			"status": 403, "detail": map[string]any{"field": "q", "value": "secret"}})
		return string(b)
	}
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Query().Get("query"))
		switch {
		case r.URL.Path == "/loki/api/v1/query":
			_, _ = w.Write([]byte(`{"data":{"result":[{"metric":{"type":"request.blocked"},"value":[1,"12"]}]}}`))
		case strings.HasPrefix(r.URL.Query().Get("query"), "sum("):
			ts := now.Truncate(time.Hour).Unix()
			_, _ = w.Write([]byte(`{"data":{"result":[{"values":[[` + itoa(ts) + `,"5"]]}]}}`))
		default:
			t1, t2 := now.Add(-time.Minute), now.Add(-2*time.Minute)
			res := map[string]any{"data": map[string]any{"result": []any{
				map[string]any{"values": [][2]string{{itoa(t2.UnixNano()), line(t2, "acme", "field.rejected")}}},
				map[string]any{"values": [][2]string{{itoa(t1.UnixNano()), line(t1, "acme", "request.blocked")},
					{itoa(t1.UnixNano() - 1), line(t1, "other", "request.blocked")}}},
			}}}
			_ = json.NewEncoder(w).Encode(res)
		}
	}))
	defer srv.Close()
	l := &Loki{URL: srv.URL, Now: func() time.Time { return now }}
	f := Filter{Tenant: "acme"}
	_ = f.Check()
	evs, next, err := l.Events(context.Background(), f, 0, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 2 || evs[0].Type != "request.blocked" || evs[1].Field == nil || *evs[1].Field != "q" {
		t.Fatalf("events: %+v", evs)
	}
	if next == "" {
		t.Error("a full page has a cursor")
	}
	b, _ := json.Marshal(evs)
	if strings.Contains(string(b), "secret") || strings.Contains(string(b), "other") {
		t.Errorf("leaked: %s", b)
	}
	counts, buckets, err := l.Stats(context.Background(), f)
	if err != nil || counts["request.blocked"] != 12 || len(buckets) != 24 || buckets[len(buckets)-2].Count != 5 {
		t.Fatalf("stats: %v %+v %v", counts, buckets, err)
	}

	srv.Close()
	if _, _, err := l.Events(context.Background(), f, 0, 3); err == nil {
		t.Error("Loki down: no error")
	}
}

func itoa(n int64) string { b, _ := json.Marshal(n); return string(b) }

func TestHealthCounts(t *testing.T) {
	now := time.Date(2026, 11, 18, 22, 0, 0, 0, time.UTC)
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.Query().Get("query"))
		if r.URL.Path == "/loki/api/v1/query" {
			_, _ = w.Write([]byte(`{"data":{"result":[{"metric":{"app":"portal","type":"legacy.error"},"value":[1,"3"]}]}}`))
			return
		}
		endOfToday := now.Truncate(24 * time.Hour).Add(24 * time.Hour).Unix()
		_, _ = w.Write([]byte(`{"data":{"result":[{"metric":{"app":"portal"},"values":[[` + itoa(endOfToday) + `,"4"]]}]}}`))
	}))
	defer srv.Close()
	l := &Loki{URL: srv.URL, Now: func() time.Time { return now }}
	recent, err := l.Recent(context.Background(), "acme", 15*time.Minute, "request.served", "legacy.error")
	if err != nil || recent["portal"]["legacy.error"] != 3 || !strings.Contains(seen[0], `tenant="acme"`) || !strings.Contains(seen[0], "[900s]") {
		t.Fatalf("recent: %v %v %v", recent, err, seen)
	}
	dates, daily, err := l.Daily(context.Background(), "acme", "legacy.error", "app", 7)
	if err != nil || len(dates) != 7 || daily["portal"][6] != 4 || !dates[6].Equal(now.Truncate(24*time.Hour)) {
		t.Fatalf("daily: %v %v %v", dates, daily, err)
	}
}
