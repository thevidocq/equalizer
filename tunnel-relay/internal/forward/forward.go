// Package forward receives the gateway's rebuilt requests on the control
// network and sends each one on a stream of the right connector's tunnel
// (schema/tunnel/tunnel.md, section 3).
package forward

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/thevidocq/equalizer/tunnel-relay/internal/logging"
	"github.com/thevidocq/equalizer/tunnel-relay/internal/registry"
	"github.com/thevidocq/equalizer/tunnel-relay/internal/sessions"
)

// Headers of the gateway → relay contract.
const (
	HeaderConnector   = "X-Eq-Connector"
	HeaderTenant      = "X-Eq-Tenant"
	HeaderHost        = "X-Eq-Host"
	HeaderDestination = "X-Eq-Destination"
	HeaderScheme      = "X-Eq-Scheme"
	HeaderError       = "X-Eq-Error"
)

// ResponseTimeout bounds the wait for the connector's response headers.
const ResponseTimeout = 40 * time.Second

type sessionKey struct{}

// Handler forwards gateway requests into tunnels.
type Handler struct {
	Pool  *sessions.Pool
	Token string // gateway token
	Log   *slog.Logger
	proxy *httputil.ReverseProxy
}

// New creates the handler.
func New(pool *sessions.Pool, token string, log *slog.Logger) *Handler {
	h := &Handler{Pool: pool, Token: token, Log: log}
	h.proxy = &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			dest := pr.In.Header.Get(HeaderDestination)
			pr.Out.URL.Scheme = "http" // the stream itself; the connector reads X-Eq-Scheme
			pr.Out.URL.Host = dest
			pr.Out.Host = dest
			for _, k := range []string{"Authorization", HeaderConnector, HeaderTenant, HeaderDestination} {
				pr.Out.Header.Del(k)
			}
		},
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				s, ok := ctx.Value(sessionKey{}).(*sessions.Session)
				if !ok {
					return nil, sessions.ErrUnavailable
				}
				return s.Open()
			},
			DisableKeepAlives:      true, // one stream per request
			DisableCompression:     true,
			ResponseHeaderTimeout:  ResponseTimeout,
			MaxResponseHeaderBytes: 64 << 10,
		},
		ErrorHandler: h.proxyError,
	}
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if h.Token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(h.Token)) != 1 {
		fail(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, dest := r.Header.Get(HeaderConnector), r.Header.Get(HeaderDestination)
	scheme, tenant := r.Header.Get(HeaderScheme), r.Header.Get(HeaderTenant)
	if !registry.ValidID(id) || !validDestination(dest) || (scheme != "http" && scheme != "https") || tenant == "" ||
		!validHost(r.Header.Get(HeaderHost)) {
		fail(w, http.StatusBadRequest, "bad_request")
		return
	}
	ctx := logging.WithRequestID(r.Context(), r.Header.Get(logging.HeaderRequestID))
	s, err := h.Pool.Pick(id, dest)
	if err != nil {
		reason := "connector_unavailable"
		status := http.StatusBadGateway
		if errors.Is(err, sessions.ErrNotDeclared) {
			reason, status = "destination_not_declared", http.StatusForbidden
		}
		h.Log.WarnContext(ctx, "request not forwarded",
			slog.Group("detail", slog.String("connector_id", id), slog.String("reason", reason)))
		fail(w, status, reason)
		return
	}
	if s.Tenant != tenant {
		h.Log.WarnContext(ctx, "request refused: connector of another tenant",
			slog.Group("detail", slog.String("connector_id", id), slog.String("tenant", tenant), slog.String("reason", "connector_not_owned")))
		fail(w, http.StatusForbidden, "connector_not_owned")
		return
	}
	start := time.Now()
	rec := &statusWriter{ResponseWriter: w, status: http.StatusOK}
	h.proxy.ServeHTTP(rec, r.WithContext(context.WithValue(ctx, sessionKey{}, s)))
	h.Log.DebugContext(ctx, "request forwarded", slog.Group("detail",
		slog.String("connector_id", id), slog.String("method", r.Method),
		slog.Int("status", rec.status), slog.Float64("duration_ms", float64(time.Since(start).Microseconds())/1000)))
}

func (h *Handler) proxyError(w http.ResponseWriter, r *http.Request, err error) {
	reason, status := "connector_unavailable", http.StatusBadGateway
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		reason, status = "legacy_timeout", http.StatusGatewayTimeout
	}
	h.Log.WarnContext(r.Context(), "tunnel stream failed",
		slog.String("error", err.Error()), slog.Group("detail", slog.String("reason", reason)))
	fail(w, status, reason)
}

func validDestination(d string) bool {
	host, port, err := net.SplitHostPort(d)
	return err == nil && host != "" && port != ""
}

// validHost accepts an empty value or a host name with an optional port.
func validHost(h string) bool {
	if h == "" {
		return true
	}
	if len(h) > 260 {
		return false
	}
	for _, c := range h {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '-' || c == ':') {
			return false
		}
	}
	return true
}

func fail(w http.ResponseWriter, status int, reason string) {
	w.Header().Set(HeaderError, reason)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(reason + "\n"))
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
