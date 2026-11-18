package health

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEvaluate(t *testing.T) {
	doc := []byte(`{"legacy":{"reach":{"mode":"connector","connector_id":"cn-1"}}}`)
	ok := Evaluate(Input{MapVersion: 3, Map: doc, Tunnels: map[string]int{"cn-1": 1}, Traffic: true, Served: 40, Exposure: "closed"})
	if ok.State != OK {
		t.Errorf("all good: %+v", ok)
	}
	cases := map[string]struct {
		in    Input
		state string
	}{
		"connector offline": {Input{MapVersion: 3, Map: doc, Tunnels: map[string]int{}, Traffic: true, Served: 4, Exposure: "closed"}, Down},
		"legacy errors":     {Input{MapVersion: 3, Map: doc, Tunnels: map[string]int{"cn-1": 1}, Traffic: true, Served: 9, Errors: 2, Exposure: "closed"}, Degraded},
		"exposed":           {Input{MapVersion: 3, Map: doc, Tunnels: map[string]int{"cn-1": 1}, Traffic: true, Served: 9, Exposure: "exposed"}, Down},
		"no map":            {Input{Traffic: true}, Down},
		"nothing known":     {Input{MapVersion: 1, Map: []byte(`{"legacy":{"reach":{"mode":"direct"}}}`)}, OK},
	}
	for name, c := range cases {
		if got := Evaluate(c.in); got.State != c.state {
			t.Errorf("%s: %s, want %s (%+v)", name, got.State, c.state, got.Checks)
		}
	}
	if Evaluate(Input{MapVersion: 1, Map: []byte(`{"legacy":{"reach":{"mode":"direct"}}}`)}).Checks[1].State != NotUsed {
		t.Error("direct mode: connector not used")
	}
}

func TestProbe(t *testing.T) {
	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/tunnels":
			_, _ = w.Write([]byte(`{"tunnels":[{"connector_id":"cn-1"},{"connector_id":"cn-1"},{"connector_id":"cn-2"}]}`))
		case "/readyz":
			w.WriteHeader(200)
		}
	}))
	defer relay.Close()
	_, port, _ := net.SplitHostPort(relay.Listener.Addr().String())
	p := &Probe{RelayAdmin: "tunnel-relay:" + port, Client: &http.Client{Timeout: time.Second},
		Resolve: func(context.Context, string) ([]string, error) { return []string{"127.0.0.1", "127.0.0.2"}, nil }}
	tunnels, ok := p.Tunnels(context.Background())
	if !ok || tunnels["cn-1"] != 2 || tunnels["cn-2"] != 1 {
		t.Fatalf("tunnels: %v %v", tunnels, ok)
	}
	if up, total := p.Replicas(context.Background(), p.RelayAdmin); total != 2 || up < 1 {
		t.Errorf("replicas: %d/%d", up, total)
	}
	if _, ok := (&Probe{}).Tunnels(context.Background()); ok {
		t.Error("no relay configured: not ok")
	}
}
