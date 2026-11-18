package tunnel

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/hashicorp/yamux"
)

// fakeRelay accepts tunnels and hands the yamux sessions to the test.
type fakeRelay struct {
	token    string
	sessions chan *yamux.Session
	headers  chan http.Header
}

func (f *fakeRelay) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer "+f.token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	f.headers <- r.Header.Clone()
	raw, brw, err := w.(http.Hijacker).Hijack()
	if err != nil {
		return
	}
	_, _ = brw.WriteString("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: " + Protocol + "\r\n\r\n")
	_ = brw.Flush()
	mux, err := yamux.Server(&buffered{Conn: raw, r: brw.Reader}, nil)
	if err != nil {
		return
	}
	f.sessions <- mux
}

type buffered struct {
	net.Conn
	r *bufio.Reader
}

func (b *buffered) Read(p []byte) (int, error) { return b.r.Read(p) }

func roundTrip(t *testing.T, mux *yamux.Session) string {
	t.Helper()
	stream, err := mux.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	req, _ := http.NewRequest("GET", "http://10.0.0.5:8080/index.jsp", nil)
	if err := req.Write(stream); err != nil {
		t.Fatal(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(stream), req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	return string(body)
}

func TestClientServesAndReconnects(t *testing.T) {
	relay := &fakeRelay{token: "tok", sessions: make(chan *yamux.Session, 4), headers: make(chan http.Header, 4)}
	srv := httptest.NewServer(relay)
	defer srv.Close()
	u, _ := url.Parse(srv.URL)

	ups := make(chan bool, 8)
	c := &Client{
		RelayURL: u, ID: "cn-1", Token: "tok", Version: "test", Allow: []string{"10.0.0.5:8080"},
		HTTP: http.DefaultClient,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "served "+r.Host+r.URL.Path)
		}),
		Log:       slog.New(slog.NewJSONHandler(io.Discard, nil)),
		Connected: func(up bool) { ups <- up },
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	h := <-relay.headers
	if h.Get("Upgrade") != Protocol || h.Get("X-Eq-Connector") != "cn-1" || h.Get("X-Eq-Allow") != "10.0.0.5:8080" {
		t.Fatalf("handshake headers: %v", h)
	}
	mux := <-relay.sessions
	if got := roundTrip(t, mux); got != "served 10.0.0.5:8080/index.jsp" {
		t.Fatalf("got %q", got)
	}

	// The relay drops the tunnel: the connector must come back.
	mux.Close()
	select {
	case mux = <-relay.sessions:
	case <-time.After(5 * time.Second):
		t.Fatal("no reconnection")
	}
	if got := roundTrip(t, mux); got != "served 10.0.0.5:8080/index.jsp" {
		t.Fatalf("after reconnection: %q", got)
	}
	if <-ups != true || <-ups != false || <-ups != true {
		t.Error("Connected callbacks out of order")
	}
}

func TestClientRejected(t *testing.T) {
	relay := &fakeRelay{token: "other"}
	srv := httptest.NewServer(relay)
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	c := &Client{RelayURL: u, ID: "cn-1", Token: "tok", HTTP: http.DefaultClient, Log: slog.New(slog.NewJSONHandler(io.Discard, nil))}
	if err := c.once(context.Background()); err == nil {
		t.Fatal("a refused tunnel must return an error")
	}
}
