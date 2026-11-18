package mapstore

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPSource(t *testing.T) {
	version := `"v1"`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Header.Get("If-None-Match") == version {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", version)
		_, _ = w.Write([]byte(`{"version":1}`))
	}))
	defer srv.Close()
	src := &HTTPSource{URL: srv.URL, Token: "tok", Client: srv.Client()}
	ctx := context.Background()
	if doc, err := src.Fetch(ctx); err != nil || string(doc) != `{"version":1}` {
		t.Fatalf("first fetch: %q %v", doc, err)
	}
	if _, err := src.Fetch(ctx); !errors.Is(err, ErrUnchanged) {
		t.Fatalf("second fetch: %v", err)
	}
	src.Forget()
	if _, err := src.Fetch(ctx); err != nil {
		t.Fatalf("after Forget: %v", err)
	}
	version = `"v2"`
	if _, err := src.Fetch(ctx); err != nil {
		t.Fatalf("new version: %v", err)
	}
	bad := &HTTPSource{URL: srv.URL, Token: "wrong", Client: srv.Client()}
	if _, err := bad.Fetch(ctx); err == nil {
		t.Fatal("wrong token accepted")
	}
}
