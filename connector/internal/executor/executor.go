// Package executor runs, towards the legacy application, the requests that
// arrive on the tunnel's streams (schema/tunnel/tunnel.md, section 2).
package executor

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/thevidocq/equalizer/connector/internal/allowlist"
	"github.com/thevidocq/equalizer/connector/internal/logging"
)

// Timeout bounds one request to the legacy application.
const Timeout = 30 * time.Second

// HeaderError carries the error reason back to the relay.
const HeaderError = "X-Eq-Error"

// ErrNotAllowed is returned by the dialer for destinations outside the allowlist.
var ErrNotAllowed = errors.New("destination_not_allowed")

// Executor is the http.Handler served on the tunnel.
type Executor struct {
	allow *allowlist.List
	log   *slog.Logger
	proxy *httputil.ReverseProxy
}

// New creates an executor. tlsConfig applies to https destinations.
func New(allow *allowlist.List, tlsConfig *tls.Config, log *slog.Logger) *Executor {
	e := &Executor{allow: allow, log: log}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	e.proxy = &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			dest := pr.In.Host
			pr.Out.URL.Scheme = pr.In.Header.Get("X-Eq-Scheme")
			pr.Out.URL.Host = dest
			pr.Out.Host = dest
			for k := range pr.Out.Header {
				if strings.HasPrefix(strings.ToLower(k), "x-eq-") {
					pr.Out.Header.Del(k)
				}
			}
		},
		Transport: &http.Transport{
			// Second barrier: whatever the request says, only allowed
			// destinations can be dialled.
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				if !allow.Contains(addr) {
					return nil, ErrNotAllowed
				}
				return dialer.DialContext(ctx, network, addr)
			},
			TLSClientConfig:       tlsConfig,
			Proxy:                 nil, // the legacy application is on the local network
			ResponseHeaderTimeout: Timeout,
			MaxIdleConnsPerHost:   8,
			IdleConnTimeout:       60 * time.Second,
			DisableCompression:    true,
		},
		ErrorHandler: e.failed,
	}
	return e
}

func (e *Executor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := logging.WithRequestID(r.Context(), r.Header.Get(logging.HeaderRequestID))
	if !e.allow.Contains(r.Host) {
		e.log.WarnContext(ctx, "destination not allowed", slog.Group("detail", slog.String("destination", printable(r.Host))))
		fail(w, http.StatusForbidden, "destination_not_allowed")
		return
	}
	if s := r.Header.Get("X-Eq-Scheme"); s != "http" && s != "https" {
		fail(w, http.StatusBadRequest, "bad_request")
		return
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	start := time.Now()
	rec := &statusWriter{ResponseWriter: w, status: http.StatusOK}
	e.proxy.ServeHTTP(rec, r.WithContext(ctx))
	e.log.DebugContext(ctx, "request executed", slog.Group("detail",
		slog.String("destination", r.Host), slog.String("method", r.Method), slog.Int("status", rec.status),
		slog.Float64("duration_ms", float64(time.Since(start).Microseconds())/1000)))
}

func (e *Executor) failed(w http.ResponseWriter, r *http.Request, err error) {
	reason, status := "legacy_unreachable", http.StatusBadGateway
	var ne net.Error
	switch {
	case errors.Is(err, ErrNotAllowed):
		reason, status = "destination_not_allowed", http.StatusForbidden
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &ne) && ne.Timeout():
		reason, status = "legacy_timeout", http.StatusGatewayTimeout
	}
	e.log.WarnContext(r.Context(), "legacy request failed",
		slog.String("error", err.Error()), slog.Group("detail", slog.String("destination", r.Host), slog.String("reason", reason)))
	fail(w, status, reason)
}

func fail(w http.ResponseWriter, status int, reason string) {
	w.Header().Set(HeaderError, reason)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(reason + "\n"))
}

func printable(s string) string {
	if len(s) > 128 {
		s = s[:128]
	}
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
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
