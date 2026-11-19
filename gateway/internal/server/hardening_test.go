package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	areabody "github.com/thevidocq/equalizer/gateway/internal/rebuild"
)

func TestSafeReturn(t *testing.T) {
	for in, want := range map[string]string{
		"/wp-admin/?x=1":         "/wp-admin/?x=1",
		"//evil.example":         "/",
		"/\\evil.example":        "/",
		"\\/evil.example":        "/",
		"/\t/evil.example":       "/",
		"/\n/evil.example":       "/",
		"/\x00/evil.example":     "/",
		"https://evil.example/":  "/",
		"/\\\\evil.example":      "/",
		"/.eq/oidc/callback?x=1": "/",
	} {
		if got := safeReturn(in); got != want {
			t.Errorf("safeReturn(%q) = %q, want %q", in, got, want)
		}
	}
}

// Paths that some legacy server reads as a way up.
func TestPathsThatClimb(t *testing.T) {
	for _, p := range []string{"/a/../b", "/a/..;/manager/html", "/a/.;/b", "/a/.. /web.config", "/a/.../b",
		"/a/..\\..\\web.config", "/a\\b", "/./a"} {
		if cleanPath(p) {
			t.Errorf("cleanPath(%q) = true", p)
		}
	}
	for _, p := range []string{"/", "/wp-admin/", "/a/b.c/d", "/file.tar.gz", "/v1.2/x", "/a;jsessionid=1/b", "/..a/b", "/a../b"} {
		if !cleanPath(p) {
			t.Errorf("cleanPath(%q) = false", p)
		}
	}
	for _, f := range []string{"a/.. /b.css", "a/.../b.css", "../b.css", "a/./b.css"} {
		if validAssetFile(f) {
			t.Errorf("validAssetFile(%q) = true", f)
		}
	}
	if !validAssetFile("css/site.min.css") {
		t.Error("ordinary asset refused")
	}
}

func TestBodiesThatExhaustMemory(t *testing.T) {
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "text/plain")
	}))
	defer legacy.Close()
	p := areaGateway(t, legacy.URL, "GET", "POST")
	post := func(body string) int {
		req := httptest.NewRequest("POST", "/form", strings.NewReader(body))
		req.Host = "blog.example"
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		p.Handler().ServeHTTP(rec, req)
		return rec.Code
	}
	if code := post(strings.Repeat("a=1&", areabody.MaxFields+1)); code != http.StatusBadRequest {
		t.Errorf("too many fields: %d", code)
	}
	if code := post("a=1&b=2"); code != http.StatusOK || bodyMemory.used != 0 {
		t.Errorf("ordinary form: %d, memory still held %d", code, bodyMemory.used)
	}
	saved := bodyMemory
	defer func() { bodyMemory = saved }()
	bodyMemory = &memBudget{max: 8}
	if code := post("a=1&b=2"); code != http.StatusServiceUnavailable {
		t.Errorf("over the memory budget: %d", code)
	}
}

func TestPanicAnswers500(t *testing.T) {
	h := guard(slog.New(slog.NewJSONHandler(io.Discard, nil)), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "boom") {
		t.Fatalf("%d %q", rec.Code, rec.Body.String())
	}
}

func TestNewSessionsPerAddress(t *testing.T) {
	l := &keyLimiter{rate: 1, burst: 3, max: 2, buckets: map[string]*bucket{}}
	now := time.Now()
	for i := 0; i < 3; i++ {
		if !l.allow("a", now) {
			t.Fatalf("request %d refused within the burst", i)
		}
	}
	if l.allow("a", now) {
		t.Error("past the burst")
	}
	if !l.allow("a", now.Add(1100*time.Millisecond)) {
		t.Error("refilled after a second")
	}
	l.allow("b", now)
	l.allow("c", now) // over max keys: the map never grows past it
	if len(l.buckets) > 2 {
		t.Errorf("%d keys remembered", len(l.buckets))
	}
}
