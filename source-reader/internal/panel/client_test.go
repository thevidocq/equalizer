package panel

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer good-token" {
			w.WriteHeader(401)
			return
		}
		got = append(got, r.URL.Path)
		switch r.URL.Path {
		case "/v1/source/claim":
			if len(got) == 2 {
				w.WriteHeader(204)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 7, "method": "sftp", "host": "web", "path": "/var/www"})
		default:
			w.WriteHeader(204)
		}
	}))
	defer srv.Close()
	c := &Client{Base: srv.URL + "/v1", Token: "good-token"}
	ctx := context.Background()
	if err := c.Hello(ctx, "1.0", "ssh-ed25519 AAAA"); err != nil {
		t.Fatal(err)
	}
	if j, err := c.Claim(ctx); err != nil || j != nil {
		t.Fatalf("nothing queued: %v %v", j, err)
	}
	j, err := c.Claim(ctx)
	if err != nil || j == nil || j.ID != 7 || *j.Host != "web" {
		t.Fatalf("job: %+v %v", j, err)
	}
	if err := c.Result(ctx, 7, nil, "boom"); err != nil {
		t.Fatal(err)
	}
	if got[len(got)-1] != "/v1/source/reads/7/result" {
		t.Fatalf("paths: %v", got)
	}
	bad := &Client{Base: srv.URL + "/v1", Token: "old"}
	if err := bad.Hello(ctx, "1.0", ""); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("401: %v", err)
	}
}
