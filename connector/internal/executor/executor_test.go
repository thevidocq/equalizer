package executor

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/thevidocq/equalizer/connector/internal/allowlist"
)

func TestExecutor(t *testing.T) {
	var seen *http.Request
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/elsewhere", http.StatusFound)
			return
		}
		w.Header().Set("Server", "Apache/2.2")
		_, _ = io.WriteString(w, "legacy page")
	}))
	defer legacy.Close()
	u, _ := url.Parse(legacy.URL)
	allow, _ := allowlist.Parse(u.Host)
	e := New(allow, nil, slog.New(slog.NewJSONHandler(io.Discard, nil)))

	do := func(host, path, scheme string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", path, nil)
		req.Host = host
		req.Header.Set("X-Eq-Scheme", scheme)
		req.Header.Set("X-Eq-Request-Id", "r-1")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}

	rec := do(u.Host, "/index.jsp?a=1", "http")
	if rec.Code != 200 || rec.Body.String() != "legacy page" || rec.Header().Get("Server") != "Apache/2.2" {
		t.Fatalf("response: %d %q %v", rec.Code, rec.Body.String(), rec.Header())
	}
	if seen.URL.RequestURI() != "/index.jsp?a=1" || seen.Header.Get("X-Eq-Scheme") != "" || seen.Header.Get("X-Eq-Request-Id") != "" {
		t.Errorf("legacy saw %s %v", seen.URL.RequestURI(), seen.Header)
	}
	// The gateway translates redirects: the connector never follows them.
	if rec := do(u.Host, "/redirect", "http"); rec.Code != http.StatusFound || !strings.HasSuffix(rec.Header().Get("Location"), "/elsewhere") {
		t.Errorf("redirect: %d %v", rec.Code, rec.Header())
	}
	for _, c := range []struct {
		host, scheme string
		status       int
		reason       string
	}{
		{"169.254.169.254:80", "http", 403, "destination_not_allowed"},
		{"127.0.0.1:22", "http", 403, "destination_not_allowed"},
		{u.Host, "gopher", 400, "bad_request"},
	} {
		rec := do(c.host, "/", c.scheme)
		if rec.Code != c.status || rec.Header().Get(HeaderError) != c.reason {
			t.Errorf("%s %s: %d %q", c.host, c.scheme, rec.Code, rec.Header().Get(HeaderError))
		}
	}

	legacy.Close()
	if rec := do(u.Host, "/", "http"); rec.Code != http.StatusBadGateway || rec.Header().Get(HeaderError) != "legacy_unreachable" {
		t.Errorf("legacy down: %d %q", rec.Code, rec.Header().Get(HeaderError))
	}
}
