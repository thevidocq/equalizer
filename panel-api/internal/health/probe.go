// Package health builds the status page: what each site needs to work
// (published map, connector, legacy application, isolation) and whether
// the platform's own services answer. It reads, it never changes anything.
package health

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// States, as in the contract (HealthState).
const (
	OK       = "ok"
	Degraded = "degraded"
	Down     = "down"
	Unknown  = "unknown"
	NotUsed  = "not_used"
)

// Probe reaches the services on the control network. Every replica of a
// service is found through DNS (the Compose service name).
type Probe struct {
	RelayAdmin   string // host:port of the relays' admin endpoint, e.g. tunnel-relay:9091
	GatewayAdmin string // e.g. gateway:9090
	LokiURL      string
	Client       *http.Client
	Resolve      func(ctx context.Context, host string) ([]string, error) // tests
}

func (p *Probe) client() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return &http.Client{Timeout: 3 * time.Second}
}

// replicas returns host:port for each address of the service name.
func (p *Probe) replicas(ctx context.Context, hostport string) ([]string, error) {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		return nil, err
	}
	resolve := p.Resolve
	if resolve == nil {
		resolve = net.DefaultResolver.LookupHost
	}
	ips, err := resolve(ctx, host)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(ips))
	for i, ip := range ips {
		out[i] = net.JoinHostPort(ip, port)
	}
	return out, nil
}

func (p *Probe) get(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := p.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New(resp.Status)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// Tunnels counts the open tunnels per connector over all relay replicas.
// ok is false when no replica answered.
func (p *Probe) Tunnels(ctx context.Context) (map[string]int, bool) {
	info, ok := p.TunnelVersions(ctx)
	if !ok {
		return nil, false
	}
	out := map[string]int{}
	for id, vs := range info {
		out[id] = len(vs)
	}
	return out, true
}

// TunnelVersions lists, per connector, the release of each open tunnel.
func (p *Probe) TunnelVersions(ctx context.Context) (map[string][]string, bool) {
	if p.RelayAdmin == "" {
		return nil, false
	}
	addrs, err := p.replicas(ctx, p.RelayAdmin)
	if err != nil {
		return nil, false
	}
	out, answered := map[string][]string{}, false
	for _, a := range addrs {
		var snap struct {
			Tunnels []struct {
				ConnectorID string `json:"connector_id"`
				Version     string `json:"version"`
			} `json:"tunnels"`
		}
		if p.get(ctx, "http://"+a+"/v1/tunnels", &snap) != nil {
			continue
		}
		answered = true
		for _, t := range snap.Tunnels {
			out[t.ConnectorID] = append(out[t.ConnectorID], t.Version)
		}
	}
	return out, answered
}

// Replicas reports how many replicas of a service are ready (/readyz) and
// how many are running (/healthz). A gateway with no published map runs but
// is not ready: that is a fresh installation, not a failure.
func (p *Probe) Replicas(ctx context.Context, hostport string) (ready, running, total int) {
	addrs, err := p.replicas(ctx, hostport)
	if err != nil {
		return 0, 0, 0
	}
	for _, a := range addrs {
		if p.get(ctx, "http://"+a+"/readyz", nil) == nil {
			ready++
			running++
		} else if p.get(ctx, "http://"+a+"/healthz", nil) == nil {
			running++
		}
	}
	return ready, running, len(addrs)
}

// ReplicaCheck turns replica counts into a check. Replicas that run but are
// not ready (a gateway with no site loaded yet) are not a failure.
func ReplicaCheck(name string, ready, running, total int) Check {
	c := Check{Name: name, Detail: fmt.Sprintf("%d of %d replicas ready", ready, total)}
	switch {
	case total == 0 || running == 0:
		c.State = Down
		c.Detail = fmt.Sprintf("0 of %d replicas answer", total)
	case ready == 0 && running == total:
		c.State = OK
		c.Detail = fmt.Sprintf("%d of %d replicas running, no site loaded yet", running, total)
	case running < total || ready < total:
		c.State = Degraded
	default:
		c.State = OK
	}
	return c
}

// LokiReady reports whether the log store answers.
func (p *Probe) LokiReady(ctx context.Context) bool {
	if p.LokiURL == "" {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.LokiURL+"/ready", nil)
	if err != nil {
		return false
	}
	resp, err := p.client().Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// Versions are the releases the replicas of a service report on /healthz,
// each with how many replicas run it.
func (p *Probe) Versions(ctx context.Context, hostport string) map[string]int {
	out := map[string]int{}
	addrs, err := p.replicas(ctx, hostport)
	if err != nil {
		return out
	}
	for _, a := range addrs {
		var h struct {
			Version string `json:"version"`
		}
		if p.get(ctx, "http://"+a+"/healthz", &h) == nil && h.Version != "" {
			out[h.Version]++
		}
	}
	return out
}
