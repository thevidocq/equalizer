package exposure

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPublicIP(t *testing.T) {
	for ip, want := range map[string]bool{
		"8.8.8.8": true, "2a00:1450::1": true,
		"10.0.0.1": false, "172.18.0.5": false, "192.168.1.1": false, "127.0.0.1": false, "169.254.169.254": false,
		"100.64.1.1": false, "0.0.0.0": false, "::1": false, "fd00::1": false, "203.0.113.9": false, "x.example": false,
		"::ffff:10.0.0.1": false,
	} {
		if _, ok := PublicIP(ip); ok != want {
			t.Errorf("PublicIP(%q) = %v, want %v", ip, ok, want)
		}
	}
}

// server answers like a web server with virtual hosts: the protected site
// for its name, a default site for everything else.
func server(t *testing.T, site http.HandlerFunc) *Prober {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.Host, "portal.example.org") {
			site(w, r)
			return
		}
		_, _ = w.Write([]byte("<html><title>Blog</title>default site</html>"))
	}))
	t.Cleanup(srv.Close)
	addr := srv.Listener.Addr().String()
	return &Prober{Timeout: 2 * time.Second, Dial: func(ctx context.Context, network, target string) (net.Conn, error) {
		if strings.HasSuffix(target, ":443") {
			return nil, &net.OpError{Op: "dial", Err: errRefused{}}
		}
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	}}
}

type errRefused struct{}

func (errRefused) Error() string   { return "connection refused" }
func (errRefused) Timeout() bool   { return false }
func (errRefused) Temporary() bool { return false }

func TestCheck(t *testing.T) {
	cases := map[string]struct {
		site http.HandlerFunc
		want Result
	}{
		"still served": {func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("<html><title>Permits portal</title>login</html>"))
		}, Result{Exposed, ReasonHTTP}},
		"restricted to the connector": {func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "forbidden", http.StatusForbidden)
		}, Result{Closed, ReasonRefused}},
		"vhost removed": {nil, Result{Closed, ReasonRefused}},
		"redirect to the protected name": {func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "https://portal.example.org/", http.StatusMovedPermanently)
		}, Result{Closed, ReasonRefused}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			site := c.site
			if site == nil { // the server shows its default site for this name too
				site = func(w http.ResponseWriter, _ *http.Request) {
					_, _ = w.Write([]byte("<html><title>Blog</title>default site</html>"))
				}
			}
			got := server(t, site).Check(context.Background(), "8.8.8.8", "portal.example.org")
			if got != c.want {
				t.Fatalf("got %+v, want %+v", got, c.want)
			}
		})
	}
	if got := (&Prober{}).Check(context.Background(), "10.0.0.1", "portal.example.org"); got.Status != Error {
		t.Fatalf("private IP probed: %+v", got)
	}
}
