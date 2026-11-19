package legacyclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/placeholder"
	"github.com/thevidocq/equalizer/gateway/internal/transport"
)

func TestBuildAndDo(t *testing.T) {
	var got *http.Request
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		w.Header().Set("Server", "Apache-Coyote/1.1")
		w.Header().Set("X-Powered-By", "Servlet/2.5")
		w.Header().Set("Set-Cookie", "JSESSIONID=abc")
		w.Header().Set("Content-Type", "text/html; charset=iso-8859-1")
		_, _ = w.Write([]byte("<html>ok</html>"))
	}))
	defer legacy.Close()
	u, _ := url.Parse(legacy.URL)

	rt, err := transport.Direct([]string{u.Host}, "", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	c, err := New(legacy.URL+"/app", rt, "eq-test", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	lr := mapmodel.LegacyRequest{
		Method: "GET",
		Path:   "/permits/{{params.id}}/detail.jsp",
		Query:  map[string]string{"st": "{{query.status}}", "p": "{{query.page}}", "fixed": "1"},
	}
	req, err := c.Build(t.Context(), lr, placeholder.Values{
		"params": {"id": "4/2"},
		"query":  {"page": "3"},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	if got.URL.EscapedPath() != "/app/permits/4%2F2/detail.jsp" {
		t.Errorf("path %q: a / inside a value must not become a separator", got.URL.EscapedPath())
	}
	if got.URL.Query().Get("p") != "3" || got.URL.Query().Get("fixed") != "1" || got.URL.Query().Has("st") {
		t.Errorf("unexpected query: %q", got.URL.RawQuery)
	}
	if got.Header.Get("User-Agent") != "eq-test" || got.Header.Get("Cookie") != "" {
		t.Errorf("unexpected headers towards the legacy application: %v", got.Header)
	}
	if resp.Header.Get("Server") != "" || resp.Header.Get("X-Powered-By") != "" || resp.Header.Get("Set-Cookie") != "" {
		t.Errorf("legacy headers not filtered: %v", resp.Header)
	}
	if resp.Header.Get("Content-Type") == "" || string(resp.Body) != "<html>ok</html>" {
		t.Errorf("unexpected response: %v %q", resp.Header, resp.Body)
	}
}

func TestRedirectNotFollowed(t *testing.T) {
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login.do", http.StatusFound)
	}))
	defer legacy.Close()
	u, _ := url.Parse(legacy.URL)
	rt, _ := transport.Direct([]string{u.Host}, "", 5*time.Second)
	c, _ := New(legacy.URL, rt, "", 0)
	req, _ := c.Build(t.Context(), mapmodel.LegacyRequest{Method: "GET", Path: "/x"}, nil)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != http.StatusFound || resp.Location == "" {
		t.Fatalf("redirect followed or lost: %d %q", resp.Status, resp.Location)
	}
}

func TestTransportAllowlist(t *testing.T) {
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer legacy.Close()
	rt, _ := transport.Direct([]string{"10.255.255.1:8080"}, "", time.Second)
	c, _ := New(legacy.URL, rt, "", time.Second)
	req, _ := c.Build(t.Context(), mapmodel.LegacyRequest{Method: "GET", Path: "/"}, nil)
	if _, err := c.Do(req); !errors.Is(err, transport.ErrDestinationNotAllowed) {
		t.Fatalf("connection outside the allowlist not blocked: %v", err)
	}
}

func TestBuildPost(t *testing.T) {
	c, err := New("http://10.0.0.1:8080/app", http.DefaultTransport, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	lr := mapmodel.LegacyRequest{Method: "POST", Path: "/save.do",
		Form: map[string]string{"id": "{{params.id}}", "text": "{{fields.message}}", "kind": "SUP", "opt": "{{fields.missing}}"}}
	req, err := c.Build(context.Background(), lr, placeholder.Values{
		"params": {"id": "42"}, "fields": {"message": "caf\xe9 & more"}})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(req.Body)
	if req.Method != "POST" || req.URL.Path != "/app/save.do" || req.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
		t.Fatalf("request: %s %s %v", req.Method, req.URL, req.Header)
	}
	if string(body) != "id=42&kind=SUP&text=caf%E9+%26+more" {
		t.Errorf("body %q", body)
	}
	if _, err := c.Build(context.Background(), mapmodel.LegacyRequest{Method: "GET", Path: "/x", Form: map[string]string{"a": "b"}}, nil); err == nil {
		t.Error("a GET with a form body must be rejected")
	}
	if _, err := c.Build(context.Background(), mapmodel.LegacyRequest{Method: "POST", Path: "/x", Files: map[string]string{"doc": "{{fields.f}}"}}, nil); err == nil {
		t.Error("file uploads are not supported yet")
	}
}

// hop sends every request to another address, as the relay does in
// connector mode.
type hop struct{ to string }

func (h hop) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.URL.Host = h.to
	return http.DefaultTransport.RoundTrip(r)
}

func TestRelativeRedirectKeepsTheLegacySite(t *testing.T) {
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/login.php?back=%2Fadmin%2F")
		w.WriteHeader(http.StatusFound)
	}))
	defer legacy.Close()
	u, _ := url.Parse(legacy.URL)
	c, _ := New("http://intranet.example", hop{u.Host}, "", time.Second)
	req, _ := c.Build(t.Context(), mapmodel.LegacyRequest{Method: "GET", Path: "/admin/"}, nil)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Location != "/login.php?back=%2Fadmin%2F" {
		t.Fatalf("Location %q: resolved against the hop instead of the legacy page", resp.Location)
	}
	req, _ = c.BuildRaw(t.Context(), "GET", "/admin/", "", nil, 0, "", nil)
	s, err := c.Open(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Body.Close()
	if s.Location != "/login.php?back=%2Fadmin%2F" {
		t.Fatalf("area Location %q", s.Location)
	}
}
