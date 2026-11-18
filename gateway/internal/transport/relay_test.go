package transport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
)

const token = "relay-token-0123456789abcdef0123456789"

func TestRelayTransport(t *testing.T) {
	var seen *http.Request
	var seenBody string
	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r
		b, _ := io.ReadAll(r.Body)
		seenBody = string(b)
		switch r.URL.Path {
		case "/down":
			w.Header().Set("X-Eq-Error", "connector_unavailable")
			w.WriteHeader(502)
		case "/blocked":
			w.Header().Set("X-Eq-Error", "destination_not_allowed")
			w.WriteHeader(403)
		case "/timeout":
			w.Header().Set("X-Eq-Error", "legacy_timeout")
			w.WriteHeader(504)
		default:
			_, _ = io.WriteString(w, "legacy page")
		}
	}))
	defer relay.Close()
	cfg, err := ParseRelay(relay.URL, token)
	if err != nil {
		t.Fatal(err)
	}
	rt, err := Relay(cfg, "cn-1", []string{"10.20.0.15:8080", "legacy.local:443"}, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	do := func(method, target, body string) (*http.Response, error) {
		ctx := telemetry.WithRequestID(context.Background(), "r-abc")
		req, _ := http.NewRequestWithContext(ctx, method, target, strings.NewReader(body))
		return rt.RoundTrip(req)
	}
	resp, err := do("POST", "http://10.20.0.15:8080/app/a%2Fb/save.do?x=1", "k=v")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "legacy page" {
		t.Fatalf("body %q", b)
	}
	if seen.Method != "POST" || seen.URL.EscapedPath() != "/app/a%2Fb/save.do" || seen.URL.RawQuery != "x=1" || seenBody != "k=v" {
		t.Errorf("relay saw %s %s?%s %q", seen.Method, seen.URL.EscapedPath(), seen.URL.RawQuery, seenBody)
	}
	want := map[string]string{"Authorization": "Bearer " + token, "X-Eq-Connector": "cn-1",
		"X-Eq-Destination": "10.20.0.15:8080", "X-Eq-Scheme": "http", "X-Eq-Request-Id": "r-abc"}
	for k, v := range want {
		if seen.Header.Get(k) != v {
			t.Errorf("header %s = %q, want %q", k, seen.Header.Get(k), v)
		}
	}
	if _, err := do("GET", "https://LEGACY.local/", ""); err != nil {
		t.Errorf("https with default port: %v", err)
	} else if seen.Header.Get("X-Eq-Destination") != "legacy.local:443" || seen.Header.Get("X-Eq-Scheme") != "https" {
		t.Errorf("https destination: %v", seen.Header)
	}

	for target, wantErr := range map[string]error{
		"http://10.20.0.15:22/":          ErrDestinationNotAllowed,
		"http://169.254.169.254/":        ErrDestinationNotAllowed,
		"http://10.20.0.15:8080/down":    ErrConnectorUnavailable,
		"http://10.20.0.15:8080/blocked": ErrDestinationNotAllowed,
		"http://10.20.0.15:8080/timeout": ErrLegacyUnreachable,
	} {
		if _, err := do("GET", target, ""); !errors.Is(err, wantErr) {
			t.Errorf("%s: %v, want %v", target, err, wantErr)
		}
	}

	relay.Close()
	if _, err := do("GET", "http://10.20.0.15:8080/", ""); !errors.Is(err, ErrConnectorUnavailable) {
		t.Errorf("relay down: %v", err)
	}
}

func TestParseRelay(t *testing.T) {
	if cfg, err := ParseRelay("", ""); cfg != nil || err != nil {
		t.Errorf("empty: %v %v", cfg, err)
	}
	for _, c := range [][2]string{{"ftp://x", token}, {"http://relay:8082", "short"}, {"::", token}} {
		if _, err := ParseRelay(c[0], c[1]); err == nil {
			t.Errorf("%v accepted", c)
		}
	}
	if _, err := Relay(&RelayConfig{URL: &url.URL{Scheme: "http", Host: "r"}, Token: token}, "", []string{"a:1"}, time.Second); err == nil {
		t.Error("missing connector id accepted")
	}
}
