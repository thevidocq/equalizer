package server

import (
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

// Limits on clients that hold the server without doing anything: a body
// sent a byte at a time, an answer never read. The headers have their own
// limit (ReadHeaderTimeout).
const (
	bodyIdle    = 30 * time.Second // longest wait for the next bytes of a body
	bodyGrace   = 30 * time.Second // then a body must keep this pace on average
	bodyMinRate = 2 << 10          // bytes per second
	answerIdle  = 60 * time.Second // longest wait for the client to take the next bytes
)

// errTooSlow ends a body that does not keep the minimum pace.
var errTooSlow = errors.New("request body too slow")

// guard protects every request: a panic answers 500 and is logged instead
// of dropping the connection silently, a body must keep a minimum pace, and
// an answer the client stops reading is given up.
func guard(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gw := &deadlineWriter{ResponseWriter: w, rc: http.NewResponseController(w)}
		// A deadline left by the previous request on this connection.
		_ = gw.rc.SetWriteDeadline(time.Time{})
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if v == http.ErrAbortHandler { //nolint:errorlint // sentinel value, not an error chain
				panic(v)
			}
			log.ErrorContext(r.Context(), "panic while serving a request",
				slog.String("error", errString(v)), slog.String("stack", string(debug.Stack())))
			if !gw.wrote {
				writePage(gw, http.StatusInternalServerError, pageError)
			}
		}()
		if r.Body != nil && r.Body != http.NoBody {
			r.Body = &pacedBody{body: r.Body, rc: gw.rc, start: time.Now()}
		}
		next.ServeHTTP(gw, r)
	})
}

func errString(v any) string {
	if err, ok := v.(error); ok {
		return err.Error()
	}
	if s, ok := v.(string); ok {
		return s
	}
	return "panic"
}

// pacedBody reads a request body under a deadline renewed at each read,
// as long as the body keeps the minimum average pace.
type pacedBody struct {
	body  interface{ Read([]byte) (int, error) }
	rc    *http.ResponseController
	start time.Time
	n     int64
}

func (b *pacedBody) Read(p []byte) (int, error) {
	if el := time.Since(b.start); el > bodyGrace && float64(b.n) < bodyMinRate*el.Seconds() {
		return 0, errTooSlow
	}
	_ = b.rc.SetReadDeadline(time.Now().Add(bodyIdle)) // not supported: the server's limits apply
	n, err := b.body.Read(p)
	b.n += int64(n)
	return n, err
}

func (b *pacedBody) Close() error {
	if c, ok := b.body.(interface{ Close() error }); ok {
		return c.Close()
	}
	return nil
}

// deadlineWriter gives the client answerIdle to take each part of the
// answer; live updates (Flush) keep working.
type deadlineWriter struct {
	http.ResponseWriter
	rc    *http.ResponseController
	wrote bool
}

func (d *deadlineWriter) WriteHeader(code int) {
	d.wrote = true
	_ = d.rc.SetWriteDeadline(time.Now().Add(answerIdle))
	d.ResponseWriter.WriteHeader(code)
}

func (d *deadlineWriter) Write(b []byte) (int, error) {
	d.wrote = true
	_ = d.rc.SetWriteDeadline(time.Now().Add(answerIdle))
	return d.ResponseWriter.Write(b)
}

func (d *deadlineWriter) Flush() {
	_ = d.rc.SetWriteDeadline(time.Now().Add(answerIdle))
	_ = d.rc.Flush()
}

// Unwrap lets http.ResponseController reach the server's writer.
func (d *deadlineWriter) Unwrap() http.ResponseWriter { return d.ResponseWriter }
