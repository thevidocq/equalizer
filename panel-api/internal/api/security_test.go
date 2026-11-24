package api_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/thevidocq/equalizer/panel-api/internal/securityscan"
)

// fakeRadar is a vulnerability scanner with one key, its targets and scans.
type fakeRadar struct {
	mu       sync.Mutex
	key      string
	targets  []map[string]any
	scans    map[string]string // id → status
	started  []map[string]any  // bodies of POST …/scans
	limitHit bool
}

func (f *fakeRadar) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer "+f.key {
		w.WriteHeader(401)
		_, _ = io.WriteString(w, `{"error":{"code":"unauthorized","message":"bad key"}}`)
		return
	}
	p := strings.TrimPrefix(r.URL.Path, "/api/public/v1")
	switch {
	case r.Method == "GET" && p == "/targets":
		writeJSONTo(w, map[string]any{"data": f.targets, "next_cursor": nil})
	case r.Method == "POST" && p == "/targets":
		var in struct{ Domain string }
		_ = json.NewDecoder(r.Body).Decode(&in)
		t := map[string]any{"id": "t-" + in.Domain, "domain": in.Domain, "risk_score": 42}
		f.targets = append(f.targets, t)
		w.WriteHeader(201)
		writeJSONTo(w, map[string]any{"data": t})
	case r.Method == "POST" && strings.HasSuffix(p, "/scans"):
		if f.limitHit {
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(429)
			_, _ = io.WriteString(w, `{"error":{"code":"rate_limited","message":"20 scans per day"}}`)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.started = append(f.started, body)
		id := "s-" + strings.Split(p, "/")[2]
		f.scans[id] = "pending"
		w.WriteHeader(202)
		writeJSONTo(w, map[string]any{"data": map[string]any{"id": id, "status": "pending"}})
	case r.Method == "GET" && strings.HasPrefix(p, "/scans/"):
		id := strings.TrimPrefix(p, "/scans/")
		writeJSONTo(w, map[string]any{"data": map[string]any{"id": id, "status": f.scans[id]}})
	case r.Method == "GET" && strings.HasPrefix(p, "/targets/"):
		writeJSONTo(w, map[string]any{"data": map[string]any{"id": strings.TrimPrefix(p, "/targets/"), "risk_score": 37}})
	case r.Method == "GET" && p == "/vulnerabilities":
		if r.URL.Query().Get("status") != "open" {
			w.WriteHeader(400)
			return
		}
		writeJSONTo(w, map[string]any{"data": []map[string]any{
			{"id": "v1", "source": "nikto", "title": "SQL error in search", "severity": "high", "cvss": 7.5,
				"url": "https://www.acme.test/?q=1'", "http_method": "GET", "parameter": "q", "cve": []string{}, "cwe": []string{"CWE-89"}},
			{"id": "v2", "source": "headers", "title": "Missing HSTS", "severity": "LOW", "url": nil},
			{"id": "v3", "source": "nikto", "title": "Backup file", "severity": "medium", "url": "https://www.acme.test/old/db.sql",
				"http_method": "GET"},
			{"id": "v4", "source": "nuclei", "title": "Odd severity", "severity": "weird", "url": "https://www.acme.test/admin.php"},
		}, "next_cursor": nil})
	default:
		w.WriteHeader(404)
	}
}

func writeJSONTo(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

const securityMap = `{"meta":{"schema":"gateway-map/1.0","tenant":"x","app":"x","version":99,"status":"published"},
 "legacy":{"base_url":"https://old.acme.example","reach":{"mode":"direct","address":"8.8.8.8:443","allow":["8.8.8.8:443"]}},
 "roles":{"anonymous":"guest","list":{"guest":{}}},
 "routes":[{"id":"home","path":"/","method":"GET","roles":["guest"],"query":{"q":{"type":"string","max_length":100}},
   "legacy_request":{"method":"GET","path":"/","query":{"q":"{{query.q}}"}},"render":"faithful","origin":"human"}],
 "areas":[{"id":"site","prefix":"/old/"}]}`

func TestSecurityScans(t *testing.T) {
	e := setup(t)
	radar := &fakeRadar{key: "cvr_test_0123456789abcdef", scans: map[string]string{}}
	api := httptest.NewTLSServer(radar)
	defer api.Close()
	e.server.ScannerHTTP = api.Client()
	e.server.LookupHost = func(_ context.Context, host string) ([]string, error) {
		if host == "old.acme.example" {
			return []string{"8.8.8.8"}, nil
		}
		return []string{"10.0.0.9"}, nil
	}
	endpoint := api.URL + "/api/public/v1"

	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	boss := e.join("boss@acme.test")
	boss.must(201, "POST", "/v1/tenants/acme/invitations", map[string]string{"email": "ap@acme.test", "name": "Ap", "role": "viewer"}, nil)
	viewer := e.join("ap@acme.test")
	boss.must(201, "POST", "/v1/tenants/acme/connectors", map[string]string{"id": "cn-1"}, nil)
	boss.must(201, "POST", "/v1/tenants/acme/apps", map[string]string{"id": "portal", "name": "Portal", "public_domain": "www.acme.test"}, nil)

	// Settings: not connected, a wrong key refused, then connected; the key never comes back
	var cfg struct {
		Endpoint  *string `json:"endpoint"`
		HasAPIKey bool    `json:"has_api_key"`
		KeyHint   *string `json:"key_hint"`
	}
	boss.must(200, "GET", "/v1/tenants/acme/scanner", nil, &cfg)
	if cfg.Endpoint != nil || cfg.HasAPIKey {
		t.Fatalf("empty settings: %+v", cfg)
	}
	viewer.must(403, "GET", "/v1/tenants/acme/scanner", nil, nil)
	boss.must(422, "PUT", "/v1/tenants/acme/scanner", map[string]string{"endpoint": "http://plain.example", "api_key": radar.key}, nil)
	boss.must(424, "PUT", "/v1/tenants/acme/scanner", map[string]string{"endpoint": endpoint, "api_key": "cvr_test_wrong_key_123"}, nil)
	boss.must(200, "PUT", "/v1/tenants/acme/scanner", map[string]string{"endpoint": endpoint, "api_key": radar.key}, &cfg)
	if !cfg.HasAPIKey || cfg.KeyHint == nil || *cfg.KeyHint != "cdef" {
		t.Fatalf("settings: %+v", cfg)
	}
	boss.must(200, "PUT", "/v1/tenants/acme/scanner", map[string]string{"endpoint": endpoint}, &cfg)           // the saved key is kept
	boss.must(422, "PUT", "/v1/tenants/acme/scanner", map[string]string{"endpoint": endpoint + "/other"}, nil) // never sent to a new address

	// Nothing to scan before a published map
	boss.must(409, "POST", "/v1/apps/portal/security/scans", map[string]any{"target": "protected"}, nil)
	boss.must(201, "POST", "/v1/apps/portal/maps", map[string]any{"document": json.RawMessage(securityMap)}, nil)
	boss.must(200, "POST", "/v1/apps/portal/maps/1/submit", nil, nil)
	boss.must(200, "POST", "/v1/apps/portal/maps/1/approve", nil, nil)
	boss.must(200, "POST", "/v1/apps/portal/maps/1/publish", nil, nil)

	// Viewers read, administrators scan; the original site needs a confirmation
	viewer.must(403, "POST", "/v1/apps/portal/security/scans", map[string]any{"target": "protected"}, nil)
	boss.must(422, "POST", "/v1/apps/portal/security/scans", map[string]any{"target": "original"}, nil)
	boss.must(422, "POST", "/v1/apps/portal/security/scans", map[string]any{"target": "protected", "scan_type": "fast"}, nil)
	var sc struct {
		ID     int64
		Target string
		Domain string
		Status string
	}
	boss.must(202, "POST", "/v1/apps/portal/security/scans", map[string]any{"target": "protected"}, &sc)
	if sc.Domain != "www.acme.test" || sc.Status != "pending" {
		t.Fatalf("scan: %+v", sc)
	}
	if v, _ := radar.started[0]["require_verification"].(bool); v || radar.started[0]["scan_type"] != "standard" {
		t.Fatalf("scan request: %v", radar.started[0])
	}
	boss.must(409, "POST", "/v1/apps/portal/security/scans", map[string]any{"target": "protected"}, nil) // one at a time
	boss.must(202, "POST", "/v1/apps/portal/security/scans", map[string]any{"target": "original", "confirm": true, "scan_type": "deep"}, &sc)
	if sc.Domain != "old.acme.example" {
		t.Fatalf("original: %+v", sc)
	}

	// The runner follows the scans: still running, then completed
	runner := &securityscan.Runner{Store: e.st, Clients: e.server.ScannerFor, Log: slog.New(slog.NewJSONHandler(io.Discard, nil))}
	runner.Once(context.Background())
	radar.mu.Lock()
	radar.scans["s-t-www.acme.test"] = "completed"
	radar.scans["s-t-old.acme.example"] = "failed"
	radar.mu.Unlock()
	if _, err := e.st.DB.Exec(`UPDATE security_scans SET polled_at = NULL`); err != nil {
		t.Fatal(err)
	}
	runner.Once(context.Background())

	var ov struct {
		Configured bool
		Targets    []struct {
			Target    string
			Domain    *string
			Available bool
			Scan      *struct {
				Status        string
				FindingsCount *int `json:"findings_count"`
				RiskScore     *int `json:"risk_score"`
				Error         *string
			}
			Findings []struct {
				ID       string
				Severity string
				Map      struct {
					Verdict   string
					Entry     *string
					FieldType *string `json:"field_type"`
				}
			}
		}
	}
	viewer.must(200, "GET", "/v1/apps/portal/security", nil, &ov)
	if !ov.Configured || len(ov.Targets) != 2 {
		t.Fatalf("overview: %+v", ov)
	}
	prot, orig := ov.Targets[0], ov.Targets[1]
	if prot.Scan == nil || prot.Scan.Status != "completed" || *prot.Scan.FindingsCount != 4 || *prot.Scan.RiskScore != 37 {
		t.Fatalf("protected scan: %+v", prot.Scan)
	}
	got := map[string]string{}
	for _, f := range prot.Findings {
		got[f.ID] = f.Severity + " " + f.Map.Verdict
	}
	want := map[string]string{"v1": "high free_text", "v2": "low server", "v3": "medium area", "v4": "info refused"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("finding %s: %q, want %q", k, got[k], v)
		}
	}
	if prot.Findings[0].ID != "v1" {
		t.Errorf("most severe first: %+v", prot.Findings)
	}
	if orig.Scan == nil || orig.Scan.Status != "failed" || orig.Scan.Error == nil || len(orig.Findings) != 0 {
		t.Fatalf("original scan: %+v", orig.Scan)
	}

	// The scanner's limits reach the user as 429
	radar.mu.Lock()
	radar.limitHit = true
	radar.mu.Unlock()
	boss.must(429, "POST", "/v1/apps/portal/security/scans", map[string]any{"target": "protected"}, nil)

	// Activity records the confirmation; disconnecting keeps the scans
	var act struct{ Items []struct{ Action string } }
	boss.must(200, "GET", "/v1/audit", nil, &act)
	seen := map[string]bool{}
	for _, a := range act.Items {
		seen[a.Action] = true
	}
	for _, a := range []string{"scanner.set", "security.scan", "security.scan_completed", "security.scan_failed"} {
		if !seen[a] {
			t.Errorf("no %s in Activity", a)
		}
	}
	boss.must(204, "DELETE", "/v1/tenants/acme/scanner", nil, nil)
	viewer.must(200, "GET", "/v1/apps/portal/security", nil, &ov)
	if ov.Configured || ov.Targets[0].Scan == nil {
		t.Fatalf("after disconnecting: %+v", ov)
	}
	boss.must(409, "POST", "/v1/apps/portal/security/scans", map[string]any{"target": "protected"}, nil)

	// A site reached through the connector has no original target: its name is internal
	boss.must(201, "POST", "/v1/tenants/acme/apps", map[string]string{"id": "intranet", "name": "Intranet", "public_domain": "intra.acme.test"}, nil)
	internal := strings.Replace(securityMap, `"reach":{"mode":"direct","address":"8.8.8.8:443","allow":["8.8.8.8:443"]}`,
		`"reach":{"mode":"connector","connector_id":"cn-1","allow":["10.0.0.1:8080"]}`, 1)
	boss.must(201, "POST", "/v1/apps/intranet/maps", map[string]any{"document": json.RawMessage(internal)}, nil)
	boss.must(200, "POST", "/v1/apps/intranet/maps/1/submit", nil, nil)
	boss.must(200, "POST", "/v1/apps/intranet/maps/1/approve", nil, nil)
	boss.must(200, "POST", "/v1/apps/intranet/maps/1/publish", nil, nil)
	viewer.must(200, "GET", "/v1/apps/intranet/security", nil, &ov)
	if !ov.Targets[0].Available || ov.Targets[1].Available || ov.Targets[1].Domain != nil {
		t.Fatalf("connector site: %+v", ov.Targets)
	}
}
