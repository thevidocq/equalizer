// Package logging produces JSON logs that conform to
// schema/events/log.schema.json and security events that conform to
// schema/events/event.schema.json.
package logging

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Service is the value of the "service" field.
const Service = "tunnel-relay"

// HeaderRequestID carries the gateway's request id.
const HeaderRequestID = "X-Eq-Request-Id"

type ctxKey struct{}

// WithRequestID returns a context carrying the request id.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// New returns a JSON logger at the given level.
func New(w io.Writer, instance, level string) *slog.Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: parseLevel(level), ReplaceAttr: replaceAttr})
	return slog.New(&contextHandler{Handler: h}).With(slog.String("service", Service), slog.String("instance", instance))
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	}
	return slog.LevelInfo
}

func replaceAttr(groups []string, a slog.Attr) slog.Attr {
	if len(groups) > 0 {
		return a
	}
	switch a.Key {
	case slog.TimeKey:
		a.Key = "ts"
		a.Value = slog.StringValue(a.Value.Time().UTC().Format("2006-01-02T15:04:05.000Z07:00"))
	case slog.LevelKey:
		l := a.Value.Any().(slog.Level)
		switch {
		case l >= slog.LevelError:
			a.Value = slog.StringValue("error")
		case l >= slog.LevelWarn:
			a.Value = slog.StringValue("warn")
		case l >= slog.LevelInfo:
			a.Value = slog.StringValue("info")
		default:
			a.Value = slog.StringValue("debug")
		}
	case slog.MessageKey:
		a.Key = "msg"
	}
	return a
}

type contextHandler struct{ slog.Handler }

func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id, _ := ctx.Value(ctxKey{}).(string); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, r)
}

func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{Handler: h.Handler.WithGroup(name)}
}

// Event is a security event (subset of the schema used by the relay).
type Event struct {
	Schema    string         `json:"schema"`
	TS        string         `json:"ts"`
	Type      string         `json:"type"`
	Reason    *string        `json:"reason"`
	Service   string         `json:"service"`
	Instance  string         `json:"instance,omitempty"`
	Tenant    string         `json:"tenant"`
	App       string         `json:"app"`
	RequestID string         `json:"request_id,omitempty"`
	SrcIP     string         `json:"src_ip,omitempty"`
	Detail    map[string]any `json:"detail,omitempty"`
}

// Events writes events as JSON lines.
type Events struct {
	mu       sync.Mutex
	enc      *json.Encoder
	instance string
}

// NewEvents creates an event writer.
func NewEvents(w io.Writer, instance string) *Events {
	return &Events{enc: json.NewEncoder(w), instance: instance}
}

// Emit fills in the common fields and writes the event.
func (e *Events) Emit(ev Event) {
	if e == nil {
		return
	}
	ev.Schema, ev.Service, ev.Instance = "event/1.0", Service, e.instance
	ev.TS = time.Now().UTC().Format("2006-01-02T15:04:05.000Z07:00")
	e.mu.Lock()
	defer e.mu.Unlock()
	_ = e.enc.Encode(ev)
}

// Ptr returns a pointer to v.
func Ptr[T any](v T) *T { return &v }
