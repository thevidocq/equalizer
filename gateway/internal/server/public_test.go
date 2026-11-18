package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/clientip"
	"github.com/thevidocq/equalizer/gateway/internal/mapstore"
	"github.com/thevidocq/equalizer/gateway/internal/router"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
)

type fixedMaps struct{ a *mapstore.Active }

func (f fixedMaps) Current() *mapstore.Active { return f.a }

func newPublic(t *testing.T, a *mapstore.Active, events io.Writer) http.Handler {
	t.Helper()
	p := &Public{
		Maps:   fixedMaps{a},
		IPs:    clientip.New(nil),
		Log:    slog.New(slog.NewJSONHandler(io.Discard, nil)),
		Events: telemetry.NewEmitter(events, "test"),
	}
	return p.Handler()
}

func activeMap(t *testing.T) *mapstore.Active {
	t.Helper()
	m := &mapstore.Model{
		Meta:   mapstore.Meta{Tenant: "t", App: "a", Version: 7, Status: "published"},
		Routes: []mapstore.Route{{ID: "home", Path: "/", Method: "GET"}},
	}
	rt, err := router.New(mapstore.Entries(m))
	if err != nil {
		t.Fatal(err)
	}
	return &mapstore.Active{Model: m, Router: rt}
}

func TestPublicBlocksUnmapped(t *testing.T) {
	var events bytes.Buffer
	h := newPublic(t, activeMap(t), &events)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/wp-admin?user=admin", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, atteso 404", rec.Code)
	}
	var ev map[string]any
	if err := json.Unmarshal(events.Bytes(), &ev); err != nil {
		t.Fatalf("evento non JSON: %v", err)
	}
	if ev["type"] != "request.blocked" || ev["reason"] != "route_not_mapped" || ev["map_version"] != float64(7) {
		t.Fatalf("evento inatteso: %v", ev)
	}
	if strings.Contains(events.String(), "user=admin") {
		t.Fatal("la query string non deve finire negli eventi")
	}
	if rec.Header().Get(telemetry.HeaderRequestID) == "" {
		t.Fatal("manca il request_id nella risposta")
	}
}

func TestPublicMethods(t *testing.T) {
	h := newPublic(t, activeMap(t), io.Discard)
	cases := map[string]int{
		"TRACE":   http.StatusMethodNotAllowed,
		"CONNECT": http.StatusMethodNotAllowed,
		"OPTIONS": http.StatusNoContent,
		"POST":    http.StatusMethodNotAllowed, // "/" esiste solo in GET
		"GET":     http.StatusServiceUnavailable,
	}
	for method, want := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, "/", nil))
		if rec.Code != want {
			t.Errorf("%s /: status %d, atteso %d", method, rec.Code, want)
		}
	}
}

func TestPublicWithoutMapFailsClosed(t *testing.T) {
	h := newPublic(t, nil, io.Discard)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d, atteso 503", rec.Code)
	}
}
