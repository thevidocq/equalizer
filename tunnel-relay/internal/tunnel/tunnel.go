// Package tunnel accepts the tunnels opened by connectors: it authenticates
// the connector, switches the connection to equalizer-tunnel/1 and starts a
// yamux session on it (see schema/tunnel/tunnel.md).
package tunnel

import (
	"bufio"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/yamux"

	"github.com/thevidocq/equalizer/tunnel-relay/internal/logging"
	"github.com/thevidocq/equalizer/tunnel-relay/internal/registry"
	"github.com/thevidocq/equalizer/tunnel-relay/internal/sessions"
)

// Protocol is the Upgrade token of this protocol version.
const Protocol = "equalizer-tunnel/1"

// Path is where connectors open the tunnel.
const Path = "/v1/tunnel"

// MaxAllow limits the destinations a connector may declare.
const MaxAllow = 64

// Handler accepts tunnels.
type Handler struct {
	Registry  *registry.Registry
	Pool      *sessions.Pool
	Log       *slog.Logger
	Events    *logging.Events
	KeepAlive time.Duration // default 15 s

	failures failures // wrong tokens per address
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != Path {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet || !headerHas(r.Header, "Connection", "upgrade") || r.Header.Get("Upgrade") != Protocol {
		w.Header().Set("Upgrade", Protocol)
		http.Error(w, "upgrade required", http.StatusUpgradeRequired)
		return
	}
	id := r.Header.Get("X-Eq-Connector")
	token, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	from := remote(r)
	if !h.failures.allowed(from, time.Now()) {
		// Too many wrong tokens from this address: not even checked.
		http.Error(w, "too many attempts", http.StatusTooManyRequests)
		return
	}
	conn, ok := h.Registry.Authenticate(id, token)
	if !ok {
		if h.failures.failed(from, time.Now()) { // reported once per address while it keeps failing
			h.Log.WarnContext(r.Context(), "tunnel refused: unknown connector or wrong token",
				slog.Group("detail", slog.String("connector_id", printable(id)), slog.String("remote", from)))
			h.Events.Emit(logging.Event{Type: "request.blocked", Reason: logging.Ptr("tunnel_unauthorized"), SrcIP: from,
				Detail: map[string]any{"connector_id": printable(id)}})
		}
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	allow, err := parseAllow(r.Header.Get("X-Eq-Allow"))
	if err != nil {
		http.Error(w, "invalid X-Eq-Allow", http.StatusBadRequest)
		return
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "upgrade not supported", http.StatusInternalServerError)
		return
	}
	raw, brw, err := hj.Hijack()
	if err != nil {
		return
	}
	// The tunnel lives long: no deadline left by the HTTP server; yamux's
	// keep-alives find a dead peer.
	_ = raw.SetDeadline(time.Time{})
	_, _ = brw.WriteString("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: " + Protocol + "\r\n\r\n")
	if err := brw.Flush(); err != nil {
		raw.Close()
		return
	}
	mux, err := yamux.Server(&bufferedConn{Conn: raw, r: brw.Reader}, h.muxConfig())
	if err != nil {
		raw.Close()
		return
	}
	go refuseStreams(mux)
	s, err := h.Pool.Add(&sessions.Session{
		ConnectorID: conn.ID, Tenant: conn.Tenant, Version: printable(r.Header.Get("X-Eq-Connector-Version")),
		Allow: allow, RemoteAddr: remote(r), TokenHash: conn.TokenSHA256,
	}, mux)
	if err != nil {
		h.Log.Warn("tunnel refused: too many tunnels", slog.Group("detail", slog.String("connector_id", conn.ID)))
		mux.Close()
		return
	}
	h.Log.Info("connector connected", slog.String("tenant", conn.Tenant),
		slog.Group("detail", slog.String("connector_id", conn.ID), slog.String("version", s.Version),
			slog.Any("allow", allow), slog.String("remote", s.RemoteAddr)))
	go h.watch(s, conn)
}

func (h *Handler) watch(s *sessions.Session, conn registry.Connector) {
	<-s.Closed()
	left := h.Pool.Remove(s)
	h.Log.Info("connector disconnected", slog.String("tenant", conn.Tenant),
		slog.Group("detail", slog.String("connector_id", conn.ID), slog.Int("sessions_left", left),
			slog.Float64("duration_s", time.Since(s.Since).Seconds())))
	if left == 0 {
		h.Events.Emit(logging.Event{Type: "connector.down", Tenant: conn.Tenant,
			Detail: map[string]any{"connector_id": conn.ID}})
	}
}

func (h *Handler) muxConfig() *yamux.Config {
	cfg := yamux.DefaultConfig()
	cfg.EnableKeepAlive = true
	cfg.KeepAliveInterval = h.KeepAlive
	if cfg.KeepAliveInterval <= 0 {
		cfg.KeepAliveInterval = 15 * time.Second
	}
	cfg.ConnectionWriteTimeout = 10 * time.Second
	cfg.AcceptBacklog = 4 // the relay opens streams; it never takes the connector's
	cfg.LogOutput = io.Discard
	return cfg
}

// refuseStreams closes every stream the connector opens: requests only go
// from the relay to the connector, and unread streams would hold memory.
func refuseStreams(mux *yamux.Session) {
	for {
		st, err := mux.AcceptStream()
		if err != nil {
			return
		}
		_ = st.Close()
	}
}

// parseAllow reads "host:port,host:port".
func parseAllow(v string) ([]string, error) {
	var out []string
	for _, d := range strings.Split(v, ",") {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		host, port, err := net.SplitHostPort(d)
		if err != nil || host == "" || port == "" {
			return nil, err
		}
		out = append(out, d)
	}
	if len(out) > MaxAllow {
		return nil, http.ErrHeaderTooLong
	}
	return out, nil
}

// headerHas reports whether a comma-separated header contains token.
func headerHas(h http.Header, key, token string) bool {
	for _, v := range h.Values(key) {
		for _, t := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(t), token) {
				return true
			}
		}
	}
	return false
}

// remote returns the connector's address: the one the edge proxy added to
// X-Forwarded-For, which is the last one (the first ones are the client's
// own and could be anything), or the peer when there is no proxy.
func remote(r *http.Request) string {
	if vs := r.Header.Values("X-Forwarded-For"); len(vs) > 0 {
		all := strings.Split(strings.Join(vs, ","), ",")
		if last := strings.TrimSpace(all[len(all)-1]); last != "" {
			return printable(last)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// printable keeps log fields short and free of control characters.
func printable(s string) string {
	if len(s) > 64 {
		s = s[:64]
	}
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}

// bufferedConn reads first what the HTTP server had already buffered.
type bufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) { return c.r.Read(p) }
