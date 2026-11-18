// Package transport porta le richieste ricostruite fino al legacy.
//
// Direct collega il gateway al legacy nella stessa rete (modalità diretta).
// La modalità via connettore arriverà con tunnel-relay e userà la stessa
// interfaccia http.RoundTripper.
package transport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// ErrDestinationNotAllowed indica un tentativo di connessione fuori allowlist.
var ErrDestinationNotAllowed = errors.New("destinazione non ammessa")

// Direct restituisce un RoundTripper che si collega solo alle destinazioni
// host:porta elencate in allow (legacy.reach.allow). Niente proxy, niente
// connessioni verso altri host anche se un redirect o un DNS lo chiedessero.
func Direct(allow []string, timeout time.Duration) (http.RoundTripper, error) {
	if len(allow) == 0 {
		return nil, errors.New("allowlist delle destinazioni vuota")
	}
	allowed := make(map[string]bool, len(allow))
	for _, a := range allow {
		host, port, err := net.SplitHostPort(a)
		if err != nil {
			return nil, fmt.Errorf("destinazione %q non valida: %w", a, err)
		}
		allowed[net.JoinHostPort(strings.ToLower(host), port)] = true
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	return &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil || !allowed[net.JoinHostPort(strings.ToLower(host), port)] {
				return nil, fmt.Errorf("%w: %s", ErrDestinationNotAllowed, addr)
			}
			return dialer.DialContext(ctx, network, addr)
		},
		MaxIdleConnsPerHost:   32,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: timeout,
		TLSHandshakeTimeout:   5 * time.Second,
		ForceAttemptHTTP2:     false, // verso il legacy solo HTTP/1.1
	}, nil
}
