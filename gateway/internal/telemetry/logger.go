// Package telemetry produces JSON logs and events that conform to schema/events/.
package telemetry

import (
	"context"
	"io"
	"log/slog"
	"strings"
)

// ServiceName is the value of the "service" field in gateway logs.
const ServiceName = "gateway"

// Logger returns a JSON logger that conforms to schema/events/log.schema.json.
// The level can be changed at runtime through level.
func Logger(w io.Writer, instance string, level *slog.LevelVar) *slog.Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: replaceAttr,
	})
	return slog.New(&contextHandler{Handler: h}).With(
		slog.String("service", ServiceName),
		slog.String("instance", instance),
	)
}

// ParseLevel converts the schema's textual level into an slog level.
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// replaceAttr renames slog's standard fields to the schema's names.
func replaceAttr(groups []string, a slog.Attr) slog.Attr {
	if len(groups) > 0 {
		return a
	}
	switch a.Key {
	case slog.TimeKey:
		a.Key = "ts"
		a.Value = slog.StringValue(a.Value.Time().UTC().Format("2006-01-02T15:04:05.000Z07:00"))
	case slog.LevelKey:
		a.Value = slog.StringValue(levelName(a.Value.Any().(slog.Level)))
	case slog.MessageKey:
		a.Key = "msg"
	}
	return a
}

func levelName(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return "error"
	case l >= slog.LevelWarn:
		return "warn"
	case l >= slog.LevelInfo:
		return "info"
	default:
		return "debug"
	}
}

// contextHandler adds the request_id found in the context.
type contextHandler struct {
	slog.Handler
}

func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := RequestID(ctx); id != "" {
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
