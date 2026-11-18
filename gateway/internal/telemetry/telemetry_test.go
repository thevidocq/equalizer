package telemetry

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoggerMatchesLogSchema(t *testing.T) {
	var buf bytes.Buffer
	lv := new(slog.LevelVar)
	log := Logger(&buf, "gw-1", lv).With(slog.String("module", "test"))
	ctx := WithRequestID(t.Context(), "r-abc")
	log.WarnContext(ctx, "prova", slog.Group("detail", slog.Int("n", 1)))

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("riga non JSON: %v (%s)", err, buf.String())
	}
	allowed := map[string]bool{"ts": true, "level": true, "service": true, "instance": true, "module": true,
		"tenant": true, "app": true, "request_id": true, "trace_id": true, "span_id": true,
		"map_version": true, "msg": true, "error": true, "detail": true}
	for k := range line {
		if !allowed[k] {
			t.Errorf("campo %q non previsto da log.schema.json", k)
		}
	}
	want := map[string]string{"level": "warn", "service": "gateway", "instance": "gw-1", "msg": "prova", "request_id": "r-abc"}
	for k, v := range want {
		if line[k] != v {
			t.Errorf("%s = %v, atteso %q", k, line[k], v)
		}
	}
	if !strings.HasSuffix(line["ts"].(string), "Z") {
		t.Errorf("ts non in UTC: %v", line["ts"])
	}
}

func TestLevelIsDynamic(t *testing.T) {
	var buf bytes.Buffer
	lv := new(slog.LevelVar)
	log := Logger(&buf, "gw-1", lv)
	log.Debug("nascosto")
	lv.Set(slog.LevelDebug)
	log.Debug("visibile")
	if strings.Contains(buf.String(), "nascosto") || !strings.Contains(buf.String(), "visibile") {
		t.Fatalf("livello non applicato a caldo: %s", buf.String())
	}
}

func TestRequestIDIgnoresClient(t *testing.T) {
	var seen string
	h := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestID(r.Context())
	}))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(HeaderRequestID, "r-scelto-dal-client")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if seen == "" || seen == "r-scelto-dal-client" {
		t.Fatalf("request_id non generato dal gateway: %q", seen)
	}
	if rec.Header().Get(HeaderRequestID) != seen {
		t.Fatal("request_id non restituito nella risposta")
	}
}
