package health

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
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
	if up, _, total := p.Replicas(context.Background(), p.RelayAdmin); total != 2 || up < 1 {
		t.Errorf("replicas: %d/%d", up, total)
	}
	if _, ok := (&Probe{}).Tunnels(context.Background()); ok {
		t.Error("no relay configured: not ok")
	}
}

func TestConnectorRelease(t *testing.T) {
	if !Older("0.1.0", "0.2.0") || !Older("v0.1.9", "0.1.10") || Older("0.2.0", "0.1.0") || Older("dev", "0.2.0") || Older("0.1.0", "abc123") {
		t.Fatal("Older")
	}
	doc := []byte(`{"legacy":{"reach":{"mode":"connector","connector_id":"cn-1"}}}`)
	in := Input{MapVersion: 1, Map: doc, Tunnels: map[string]int{"cn-1": 1}, Versions: []string{"0.1.0"}, Platform: "0.2.0"}
	if c := Evaluate(in).Checks[1]; c.State != Degraded || !strings.Contains(c.Detail, "update it") {
		t.Errorf("outdated connector: %+v", c)
	}
	in.Versions = []string{"0.2.0"}
	if c := Evaluate(in).Checks[1]; c.State != OK {
		t.Errorf("current connector: %+v", c)
	}
}

func TestReplicaCheck(t *testing.T) {
	cases := []struct {
		ready, running, total int
		want                  string
	}{
		{0, 2, 2, OK}, // fresh installation: running, no site loaded yet
		{2, 2, 2, OK},
		{1, 2, 2, Degraded},
		{0, 1, 2, Degraded},
		{0, 0, 2, Down},
		{0, 0, 0, Down},
	}
	for _, c := range cases {
		if got := ReplicaCheck("Gateway", c.ready, c.running, c.total); got.State != c.want {
			t.Errorf("%d/%d/%d: %s (%s), want %s", c.ready, c.running, c.total, got.State, got.Detail, c.want)
		}
	}
	if d := ReplicaCheck("Gateway", 0, 2, 2).Detail; !strings.Contains(d, "no site loaded yet") {
		t.Errorf("detail: %q", d)
	}
}

// A site reached directly accepts Equalizer's address: a check from the
// platform proves nothing, so Isolation is never shown as fine or exposed.
func TestIsolationDirect(t *testing.T) {
	doc := []byte(`{"legacy":{"reach":{"mode":"direct","address":"8.8.8.8:443","allow":["8.8.8.8:443"]}}}`)
	for _, exp := range []string{"closed", "exposed", "not_configured"} {
		s := Evaluate(Input{MapVersion: 1, Map: doc, Exposure: exp, Traffic: true})
		for _, c := range s.Checks {
			if c.Name == "Isolation" && (c.State != Unknown || !strings.Contains(c.Detail, "another network")) {
				t.Errorf("%s: %+v", exp, c)
			}
		}
	}
}
