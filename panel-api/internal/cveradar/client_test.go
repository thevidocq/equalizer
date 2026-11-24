package cveradar

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPagesAndErrors(t *testing.T) {
	var auth []string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = append(auth, r.Header.Get("Authorization"))
		switch {
		case r.URL.Path == "/v1/vulnerabilities" && r.URL.Query().Get("cursor") == "":
			fmt.Fprint(w, `{"data":[{"id":"a","severity":"high"}],"next_cursor":"p2"}`)
		case r.URL.Path == "/v1/vulnerabilities":
			fmt.Fprint(w, `{"data":[{"id":"b","severity":"low"}],"next_cursor":null}`)
		case r.URL.Path == "/v1/targets/t1/scans":
			w.Header().Set("Retry-After", "120")
			w.WriteHeader(429)
			fmt.Fprint(w, `{"error":{"code":"rate_limited","message":"20 scans per day"}}`)
		case r.URL.Path == "/v1/scans/big":
			fmt.Fprint(w, `{"data":{"id":"`+strings.Repeat("x", maxBody)+`"}}`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	c := &Client{Endpoint: srv.URL + "/v1", Key: "cvr_test_k", HTTP: srv.Client()}
	ctx := context.Background()

	v, err := c.OpenVulnerabilities(ctx, "t1")
	if err != nil || len(v) != 2 || v[1].ID != "b" {
		t.Fatalf("pages: %v %v", v, err)
	}
	if auth[0] != "Bearer cvr_test_k" {
		t.Fatalf("auth: %q", auth[0])
	}
	_, err = c.StartScan(ctx, "t1", "standard")
	var se *Error
	if !errors.As(err, &se) || se.Status != 429 || se.Code != "rate_limited" || se.RetryAfter != 2*time.Minute {
		t.Fatalf("429: %#v", err)
	}
	if strings.Contains(err.Error(), "cvr_test_k") {
		t.Fatal("the key is in an error")
	}
	if _, err := c.GetScan(ctx, "big"); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("size cap: %v", err)
	}
}

// A tenant's address must not reach the platform's own network.
func TestPublicOnly(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"data":[]}`) }))
	defer srv.Close()
	c := &Client{Endpoint: srv.URL, Key: "k"}
	if err := c.Check(context.Background()); err == nil || !strings.Contains(err.Error(), "not a public address") {
		t.Fatalf("loopback reached: %v", err)
	}
}

func TestValidEndpoint(t *testing.T) {
	for raw, want := range map[string]bool{
		"https://api.example.com/api/public/v1": true,
		"http://api.example.com":                false,
		"https://user:pw@api.example.com":       false,
		"https://api.example.com/?x=1":          false,
		"https://":                              false,
		"ftp://api.example.com":                 false,
	} {
		if ValidEndpoint(raw) != want {
			t.Errorf("%s: want %v", raw, want)
		}
	}
	if !OnHTTPSPort("https://api.example.com:443/v1") || OnHTTPSPort("https://api.example.com:8443/v1") {
		t.Error("port 443 only")
	}
}
