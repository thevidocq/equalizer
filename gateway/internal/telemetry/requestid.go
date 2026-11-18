package telemetry

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

type ctxKey struct{}

// HeaderRequestID carries the request_id to the relay and the connector and
// back to the client. It is never accepted from the client.
const HeaderRequestID = "X-Eq-Request-Id"

// NewRequestID generates a random identifier: "r-" + 16 hex digits.
func NewRequestID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "r-" + hex.EncodeToString(b[:])
}

// WithRequestID returns a context carrying the request_id.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// RequestID reads the request_id from the context, or "" if absent.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}

// RequestIDMiddleware assigns a new request_id to every request, ignoring any
// value sent by the client, and exposes it in the response.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := NewRequestID()
		w.Header().Set(HeaderRequestID, id)
		next.ServeHTTP(w, r.WithContext(WithRequestID(r.Context(), id)))
	})
}
