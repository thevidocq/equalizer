package legacyclient

import (
	"errors"
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

	rt, err := transport.Direct([]string{u.Host}, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	c, err := New(legacy.URL+"/app", rt, "eq-test", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	lr := mapmodel.LegacyRequest{
		Method: "GET",
		Path:   "/pratiche/{{params.id}}/detail.jsp",
		Query:  map[string]string{"st": "{{query.stato}}", "p": "{{query.pagina}}", "fisso": "1"},
	}
	req, err := c.Build(t.Context(), lr, placeholder.Values{
		"params": {"id": "4/2"},
		"query":  {"pagina": "3"},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	if got.URL.EscapedPath() != "/app/pratiche/4%2F2/detail.jsp" {
		t.Errorf("percorso %q: un / in un valore non deve diventare un separatore", got.URL.EscapedPath())
	}
	if got.URL.Query().Get("p") != "3" || got.URL.Query().Get("fisso") != "1" || got.URL.Query().Has("st") {
		t.Errorf("query inattesa: %q", got.URL.RawQuery)
	}
	if got.Header.Get("User-Agent") != "eq-test" || got.Header.Get("Cookie") != "" {
		t.Errorf("header inattesi verso il legacy: %v", got.Header)
	}
	if resp.Header.Get("Server") != "" || resp.Header.Get("X-Powered-By") != "" || resp.Header.Get("Set-Cookie") != "" {
		t.Errorf("header del legacy non filtrati: %v", resp.Header)
	}
	if resp.Header.Get("Content-Type") == "" || string(resp.Body) != "<html>ok</html>" {
		t.Errorf("risposta inattesa: %v %q", resp.Header, resp.Body)
	}
}

func TestRedirectNotFollowed(t *testing.T) {
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login.do", http.StatusFound)
	}))
	defer legacy.Close()
	u, _ := url.Parse(legacy.URL)
	rt, _ := transport.Direct([]string{u.Host}, 5*time.Second)
	c, _ := New(legacy.URL, rt, "", 0)
	req, _ := c.Build(t.Context(), mapmodel.LegacyRequest{Method: "GET", Path: "/x"}, nil)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != http.StatusFound || resp.Location == "" {
		t.Fatalf("redirect seguito o perso: %d %q", resp.Status, resp.Location)
	}
}

func TestTransportAllowlist(t *testing.T) {
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer legacy.Close()
	rt, _ := transport.Direct([]string{"10.255.255.1:8080"}, time.Second)
	c, _ := New(legacy.URL, rt, "", time.Second)
	req, _ := c.Build(t.Context(), mapmodel.LegacyRequest{Method: "GET", Path: "/"}, nil)
	if _, err := c.Do(req); !errors.Is(err, transport.ErrDestinationNotAllowed) {
		t.Fatalf("connessione fuori allowlist non bloccata: %v", err)
	}
}
