// Package tunnel opens and keeps open the tunnel towards the relay
// (schema/tunnel/tunnel.md, section 1) and serves the requests that arrive
// on it.
package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/yamux"
)

// Protocol is the Upgrade token of this protocol version.
const Protocol = "equalizer-tunnel/1"

// Path is where the relay accepts tunnels.
const Path = "/v1/tunnel"

// Backoff limits between reconnection attempts.
const (
	minBackoff = time.Second
	maxBackoff = 30 * time.Second
	// A tunnel that stayed up this long resets the backoff.
	stableAfter = time.Minute
)

// ErrRejected reports that the relay refused the tunnel.
var ErrRejected = errors.New("tunnel refused by the relay")

// Client keeps one tunnel open.
type Client struct {
	RelayURL  *url.URL
	ID        string
	Token     string
	Version   string
	Allow     []string
	HTTP      *http.Client // used only for the handshake; no overall timeout
	Handler   http.Handler // serves the requests arriving on the tunnel
	Log       *slog.Logger
	KeepAlive time.Duration
	// Connected, if set, is called every time the tunnel opens or closes.
	Connected func(up bool)
}

// Run opens the tunnel and reopens it until ctx ends.
func (c *Client) Run(ctx context.Context) {
	backoff := minBackoff
	for ctx.Err() == nil {
		start := time.Now()
		err := c.once(ctx)
		if ctx.Err() != nil {
			return
		}
		if time.Since(start) > stableAfter {
			backoff = minBackoff
		}
		wait := backoff/2 + rand.N(backoff/2+1)
		attrs := []any{slog.Group("detail", slog.Float64("retry_in_s", wait.Seconds()))}
		if err != nil {
			attrs = append(attrs, slog.String("error", err.Error()))
		}
		if errors.Is(err, ErrRejected) {
			c.Log.Error("tunnel refused, check EQ_CONNECTOR_ID and EQ_CONNECTOR_TOKEN", attrs...)
		} else {
			c.Log.Warn("tunnel closed", attrs...)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		backoff = min(backoff*2, maxBackoff)
	}
}

func (c *Client) once(ctx context.Context) error {
	target := c.RelayURL.JoinPath(Path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", Protocol)
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("X-Eq-Connector", c.ID)
	req.Header.Set("X-Eq-Connector-Version", c.Version)
	req.Header.Set("X-Eq-Allow", strings.Join(c.Allow, ","))
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("connecting to the relay: %w", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		resp.Body.Close()
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusUpgradeRequired {
			return fmt.Errorf("%w: status %d", ErrRejected, resp.StatusCode)
		}
		return fmt.Errorf("relay answered %d", resp.StatusCode)
	}
	rwc, ok := resp.Body.(io.ReadWriteCloser)
	if !ok {
		resp.Body.Close()
		return errors.New("upgraded connection not available")
	}
	mux, err := yamux.Client(rwc, c.muxConfig())
	if err != nil {
		rwc.Close()
		return err
	}
	srv := &http.Server{Handler: c.Handler, ReadHeaderTimeout: 30 * time.Second}
	go func() { _ = srv.Serve(mux) }()
	c.Log.Info("tunnel open", slog.Group("detail", slog.String("relay", c.RelayURL.Host), slog.Any("allow", c.Allow)))
	if c.Connected != nil {
		c.Connected(true)
	}
	select {
	case <-mux.CloseChan():
	case <-ctx.Done():
	}
	_ = mux.Close()
	_ = srv.Close()
	if c.Connected != nil {
		c.Connected(false)
	}
	return nil
}

func (c *Client) muxConfig() *yamux.Config {
	cfg := yamux.DefaultConfig()
	cfg.EnableKeepAlive = true
	cfg.KeepAliveInterval = c.KeepAlive
	if cfg.KeepAliveInterval <= 0 {
		cfg.KeepAliveInterval = 15 * time.Second
	}
	cfg.ConnectionWriteTimeout = 10 * time.Second
	cfg.LogOutput = io.Discard
	return cfg
}
