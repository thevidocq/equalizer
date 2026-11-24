// Package logging produces JSON logs that conform to
// schema/events/log.schema.json.
package logging

import (
	"io"
	"log/slog"
	"strings"
)

// Service is the value of the "service" field.
const Service = "source-reader"

// New returns a JSON logger at the given level.
func New(w io.Writer, instance, level string) *slog.Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: parseLevel(level), ReplaceAttr: replaceAttr})
	return slog.New(h).With(slog.String("service", Service), slog.String("instance", instance))
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
