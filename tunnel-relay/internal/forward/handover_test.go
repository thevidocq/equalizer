package forward_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thevidocq/equalizer/tunnel-relay/internal/forward"
	"github.com/thevidocq/equalizer/tunnel-relay/internal/sessions"
)

// Two replicas: the connector's tunnel is on A; the gateway calls B.
func TestHandOverToTheReplicaWithTheTunnel(t *testing.T) {
	a := newRelay(t)
	connect(t, a.tunnels.URL, "secret-token", "10.0.0.5:8080", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_, _ = io.WriteString(w, "from A: "+string(b))
	}))
	waitTunnels(t, a.pool, 1)

	empty := httptest.NewServer(forward.New(sessions.NewPool(), gatewayToken, slog.New(slog.NewJSONHandler(io.Discard, nil))))
	defer empty.Close()
	b := forward.New(sessions.NewPool(), gatewayToken, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	b.Peers = func(context.Context) []string {
		return []string{strings.TrimPrefix(empty.URL, "http://"), strings.TrimPrefix(a.forward.URL, "http://")}
	}
	bsrv := httptest.NewServer(b)
	defer bsrv.Close()

	resp := send(t, bsrv.URL, nil, "k=v")
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || string(body) != "from A: k=v" {
		t.Fatalf("handover: %d %q", resp.StatusCode, body)
	}
	// Another tenant's request is refused by A as well.
	if resp := send(t, bsrv.URL, map[string]string{"X-Eq-Tenant": "other"}, ""); resp.StatusCode != 403 {
		t.Fatalf("tenant check after handover: %d", resp.StatusCode)
	}
	// No replica has it: 502, and a handed-over request is not handed over again.
	if resp := send(t, empty.URL, map[string]string{"X-Eq-Relay-Hop": "1"}, ""); resp.StatusCode != 502 {
		t.Fatalf("hop: %d", resp.StatusCode)
	}
}
