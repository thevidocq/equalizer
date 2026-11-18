package forward_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/yamux"

	"github.com/thevidocq/equalizer/tunnel-relay/internal/forward"
	"github.com/thevidocq/equalizer/tunnel-relay/internal/registry"
	"github.com/thevidocq/equalizer/tunnel-relay/internal/sessions"
	"github.com/thevidocq/equalizer/tunnel-relay/internal/tunnel"
)

const gatewayToken = "gateway-token-0123456789abcdef0123456789"

type relay struct {
	tunnels *httptest.Server
	forward *httptest.Server
	pool    *sessions.Pool
	reg     *registry.Registry
	path    string
}

func newRelay(t *testing.T) *relay {
	t.Helper()
	path := filepath.Join(t.TempDir(), "connectors.json")
	writeConnectors(t, path, "secret-token")
	reg := registry.New(path)
	if _, err := reg.Reload(); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewJSONHandler(io.Discard, nil))
	pool := sessions.NewPool()
	r := &relay{pool: pool, reg: reg, path: path,
		tunnels: httptest.NewServer(&tunnel.Handler{Registry: reg, Pool: pool, Log: log}),
		forward: httptest.NewServer(forward.New(pool, gatewayToken, log)),
	}
	t.Cleanup(func() { r.tunnels.Close(); r.forward.Close() })
	return r
}

func writeConnectors(t *testing.T, path, token string) {
	t.Helper()
	body := `{"connectors":[{"id":"cn-1","tenant":"t","app":"a","token_sha256":"` + registry.Hash(token) + `"}]}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// connect opens a tunnel like the connector does and serves h on it.
func connect(t *testing.T, url, token, allow string, h http.Handler) (*http.Response, *yamux.Session) {
	t.Helper()
	req, _ := http.NewRequest("GET", url+tunnel.Path, nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", tunnel.Protocol)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Eq-Connector", "cn-1")
	req.Header.Set("X-Eq-Allow", allow)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		return resp, nil
	}
	mux, err := yamux.Client(resp.Body.(io.ReadWriteCloser), nil)
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = (&http.Server{Handler: h}).Serve(mux) }()
	t.Cleanup(func() { mux.Close() })
	waitTunnels(t, nil, 0)
	return resp, mux
}

func waitTunnels(t *testing.T, pool *sessions.Pool, want int) {
	t.Helper()
	if pool == nil {
		time.Sleep(50 * time.Millisecond)
		return
	}
	for i := 0; i < 100; i++ {
		if len(pool.Snapshot()) == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("tunnels: %d, want %d", len(pool.Snapshot()), want)
}

func send(t *testing.T, url string, header map[string]string, body string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("POST", url+"/save.do?x=1", strings.NewReader(body))
	for k, v := range map[string]string{
		"Authorization": "Bearer " + gatewayToken, "X-Eq-Connector": "cn-1",
		"X-Eq-Destination": "10.0.0.5:8080", "X-Eq-Scheme": "http", "X-Eq-Request-Id": "r-1",
	} {
		req.Header.Set(k, v)
	}
	for k, v := range header {
		if v == "" {
			req.Header.Del(k)
		} else {
			req.Header.Set(k, v)
		}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func TestForwardThroughTunnel(t *testing.T) {
	r := newRelay(t)
	var seen *http.Request
	var seenBody string
	connect(t, r.tunnels.URL, "secret-token", "10.0.0.5:8080", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		seen = req
		b, _ := io.ReadAll(req.Body)
		seenBody = string(b)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(201)
		_, _ = io.WriteString(w, "<p>ok</p>")
	}))
	waitTunnels(t, r.pool, 1)

	resp := send(t, r.forward.URL, nil, "a=1&b=2")
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 201 || string(body) != "<p>ok</p>" {
		t.Fatalf("response %d %q", resp.StatusCode, body)
	}
	if seen.Host != "10.0.0.5:8080" || seen.URL.RequestURI() != "/save.do?x=1" || seenBody != "a=1&b=2" || seen.Method != "POST" {
		t.Fatalf("connector saw %s %s %s %q", seen.Method, seen.Host, seen.URL.RequestURI(), seenBody)
	}
	for _, h := range []string{"Authorization", "X-Eq-Connector", "X-Eq-Destination", "X-Forwarded-For"} {
		if seen.Header.Get(h) != "" {
			t.Errorf("header %s reached the connector", h)
		}
	}
	if seen.Header.Get("X-Eq-Scheme") != "http" || seen.Header.Get("X-Eq-Request-Id") != "r-1" {
		t.Errorf("scheme or request id lost: %v", seen.Header)
	}
}

func TestForwardErrors(t *testing.T) {
	r := newRelay(t)
	cases := []struct {
		name   string
		header map[string]string
		status int
		reason string
	}{
		{"no gateway token", map[string]string{"Authorization": ""}, 401, "unauthorized"},
		{"wrong gateway token", map[string]string{"Authorization": "Bearer nope"}, 401, "unauthorized"},
		{"bad destination", map[string]string{"X-Eq-Destination": "nohost"}, 400, "bad_request"},
		{"bad scheme", map[string]string{"X-Eq-Scheme": "ftp"}, 400, "bad_request"},
		{"no tunnel", nil, 502, "connector_unavailable"},
	}
	for _, c := range cases {
		resp := send(t, r.forward.URL, c.header, "")
		if resp.StatusCode != c.status || resp.Header.Get("X-Eq-Error") != c.reason {
			t.Errorf("%s: %d %q", c.name, resp.StatusCode, resp.Header.Get("X-Eq-Error"))
		}
	}

	connect(t, r.tunnels.URL, "secret-token", "10.0.0.9:80", http.NotFoundHandler())
	waitTunnels(t, r.pool, 1)
	if resp := send(t, r.forward.URL, nil, ""); resp.StatusCode != 403 || resp.Header.Get("X-Eq-Error") != "destination_not_declared" {
		t.Errorf("undeclared destination: %d %q", resp.StatusCode, resp.Header.Get("X-Eq-Error"))
	}
}

func TestTunnelAuthAndRevocation(t *testing.T) {
	r := newRelay(t)
	if resp, _ := connect(t, r.tunnels.URL, "wrong", "10.0.0.5:8080", nil); resp.StatusCode != 401 {
		t.Errorf("wrong token: %d", resp.StatusCode)
	}
	req, _ := http.NewRequest("GET", r.tunnels.URL+tunnel.Path, nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "equalizer-tunnel/9")
	if resp, err := http.DefaultClient.Do(req); err != nil || resp.StatusCode != 426 {
		t.Errorf("other version: %v %v", resp, err)
	}

	_, mux := connect(t, r.tunnels.URL, "secret-token", "10.0.0.5:8080", http.NotFoundHandler())
	waitTunnels(t, r.pool, 1)
	// Token rotated: the open tunnel must be closed on reload.
	time.Sleep(10 * time.Millisecond)
	writeConnectors(t, r.path, "new-token")
	if changed, err := r.reg.Reload(); err != nil || !changed {
		t.Fatal(err)
	}
	r.pool.CloseExcept(func(s *sessions.Session) bool {
		c, ok := r.reg.Get(s.ConnectorID)
		return ok && c.TokenSHA256 == s.TokenHash
	})
	waitTunnels(t, r.pool, 0)
	select {
	case <-mux.CloseChan():
	case <-time.After(2 * time.Second):
		t.Fatal("the connector side was not closed")
	}
}
