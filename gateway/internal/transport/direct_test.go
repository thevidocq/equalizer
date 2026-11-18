package transport

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDirectAddress(t *testing.T) {
	var host string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { host = r.Host }))
	defer srv.Close()
	addr := strings.TrimPrefix(srv.URL, "http://")
	rt, err := Direct([]string{addr}, addr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", "http://permits.example.org/", nil)
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if host != "permits.example.org" {
		t.Fatalf("Host %q: the site's name must reach the web server", host)
	}
	if _, err := Direct([]string{addr}, "10.0.0.1:80", time.Second); err == nil {
		t.Fatal("address outside allow accepted")
	}
	plain, _ := Direct([]string{addr}, "", time.Second)
	if _, err := plain.RoundTrip(req); !errors.Is(err, ErrDestinationNotAllowed) {
		t.Fatalf("without address the URL's host must be allowed: %v", err)
	}
}
