package telemetry

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

type ctxKey struct{}

// HeaderRequestID è l'header con cui il request_id viaggia verso relay e
// connettore e torna al client. Non viene mai accettato dal client.
const HeaderRequestID = "X-Eq-Request-Id"

// NewRequestID genera un identificativo casuale "r-" + 16 cifre esadecimali.
func NewRequestID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "r-" + hex.EncodeToString(b[:])
}

// WithRequestID restituisce un context con il request_id.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// RequestID legge il request_id dal context, o "" se assente.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}

// RequestIDMiddleware assegna a ogni richiesta un nuovo request_id, ignorando
// qualunque valore inviato dal client, e lo espone nella risposta.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := NewRequestID()
		w.Header().Set(HeaderRequestID, id)
		next.ServeHTTP(w, r.WithContext(WithRequestID(r.Context(), id)))
	})
}
