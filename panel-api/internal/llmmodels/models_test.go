package llmmodels

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Header.Get("x-api-key") == "bad" || r.Header.Get("Authorization") == "Bearer bad":
			w.WriteHeader(http.StatusUnauthorized)
		case r.URL.Path == "/v1/models" && r.Header.Get("anthropic-version") != "":
			_, _ = w.Write([]byte(`{"data":[{"id":"claude-new","display_name":"Claude New"},{"id":"claude-old","display_name":"Claude Old"}]}`))
		case r.URL.Path == "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4.1"},{"id":"text-embedding-3-small"},{"id":"gpt-5"},{"id":"whisper-1"},{"id":"o3"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	l := Lister{Client: srv.Client(), AnthropicURL: srv.URL, OpenAIURL: srv.URL}
	ctx := context.Background()
	got, err := l.List(ctx, "anthropic", "", "k")
	if err != nil || len(got) != 2 || got[0].ID != "claude-new" || got[0].Name != "Claude New" {
		t.Fatalf("anthropic: %v %v", got, err)
	}
	got, err = l.List(ctx, "openai", "", "k")
	if err != nil || len(got) != 3 || got[0].ID != "o3" || got[1].ID != "gpt-5" {
		t.Fatalf("openai: %v %v", got, err)
	}
	got, err = l.List(ctx, "local", srv.URL+"/v1/", "")
	if err != nil || len(got) != 5 { // a local server's list is kept whole
		t.Fatalf("local: %v %v", got, err)
	}
	if _, err := l.List(ctx, "anthropic", "", "bad"); !errors.Is(err, ErrKeyRefused) {
		t.Fatalf("bad key: %v", err)
	}
	if _, err := l.List(ctx, "local", srv.URL+"/nothing", ""); !errors.Is(err, ErrUnreachable) {
		t.Fatalf("no list: %v", err)
	}
}

// The default client never reaches a private address: a tenant's endpoint
// is not a way into the platform's network.
func TestPrivateEndpointRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"x"}]}`))
	}))
	defer srv.Close()
	if _, err := (Lister{}).List(context.Background(), "local", srv.URL+"/v1", ""); !errors.Is(err, ErrUnreachable) {
		t.Fatalf("loopback endpoint reached: %v", err)
	}
}
