package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/clientip"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
	"github.com/thevidocq/equalizer/gateway/internal/unmapped"
)

// A file the map does not allow yet is reported to discovery; a file whose
// content contradicts its type is not.
func TestRefusedAssetsReported(t *testing.T) {
	st := testSite(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/css/movie.mp4":
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write([]byte("\x00\x00\x00\x18ftypmp42"))
		case "/img/shell.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("<?php ?>"))
		}
	}))
	var got []string
	panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Items []struct{ Path string } `json:"items"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		for _, it := range body.Items {
			got = append(got, it.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer panel.Close()
	rep := &unmapped.Reporter{URL: panel.URL, Client: panel.Client()}
	p := &Public{Maps: fixedMaps{st}, IPs: clientip.New(nil), Unmapped: rep,
		Log: slog.New(slog.NewJSONHandler(io.Discard, nil)), Events: telemetry.NewEmitter(io.Discard, "test")}
	h := p.Handler()

	for _, target := range []string{"/css/movie.mp4", "/img/shell.png"} {
		if rec := get(h, "GET", target, nil); rec.Code != http.StatusNotFound {
			t.Fatalf("%s: status %d, want 404", target, rec.Code)
		}
	}
	if err := rep.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "/css/movie.mp4" {
		t.Errorf("reported %v, want only /css/movie.mp4", got)
	}
}
