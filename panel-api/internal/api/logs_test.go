package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/logsearch"
)

func TestLogs(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	boss := e.join("boss@acme.test")

	boss.must(503, "GET", "/v1/tenants/acme/logs", nil, nil) // no Loki configured

	var seen []string
	loki := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.Query().Get("query"))
		ts := time.Now().Add(-time.Minute)
		line, _ := json.Marshal(map[string]any{"ts": ts, "type": "request.blocked", "tenant": "acme", "path": "=HYPERLINK(1)", "status": 404})
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"result": []any{
			map[string]any{"values": [][2]string{{strconv.FormatInt(ts.UnixNano(), 10), string(line)}}}}}})
	}))
	defer loki.Close()
	e.server.Logs = &logsearch.Loki{URL: loki.URL}

	var page struct {
		Items []map[string]any `json:"items"`
	}
	boss.must(200, "GET", "/v1/tenants/acme/logs?types=request.blocked&q=203.0.113.4&since=7d", nil, &page)
	if len(page.Items) != 1 || !strings.Contains(seen[len(seen)-1], `tenant="acme"`) {
		t.Fatalf("logs: %+v %v", page, seen)
	}
	boss.must(422, "GET", "/v1/tenants/acme/logs?types=nope", nil, nil)
	boss.must(403, "GET", "/v1/tenants/other/logs", nil, nil)
	root.must(403, "GET", "/v1/tenants/acme/logs", nil, nil) // the platform does not read tenants' events

	req, _ := http.NewRequest("GET", e.srv.URL+"/v1/tenants/acme/logs/export", nil)
	req.Header.Set("X-Eq-Panel", "1")
	resp, err := boss.c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), "'=HYPERLINK(1)") {
		t.Fatalf("export: %d %s", resp.StatusCode, body)
	}
}
