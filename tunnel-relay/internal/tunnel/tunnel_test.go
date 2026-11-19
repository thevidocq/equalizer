package tunnel

import (
	"net"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/yamux"
)

func TestRemoteIsTheProxysAddress(t *testing.T) {
	r := httptest.NewRequest("GET", Path, nil)
	r.RemoteAddr = "10.0.0.2:4000"
	r.Header.Add("X-Forwarded-For", "6.6.6.6") // written by the client
	r.Header.Add("X-Forwarded-For", "203.0.113.9")
	if got := remote(r); got != "203.0.113.9" {
		t.Errorf("remote = %s", got)
	}
	r.Header.Del("X-Forwarded-For")
	if got := remote(r); got != "10.0.0.2" {
		t.Errorf("without proxy: %s", got)
	}
}

func TestWrongTokensFromOneAddress(t *testing.T) {
	var f failures
	now := time.Now()
	if !f.failed("a", now) {
		t.Error("the first failure is reported")
	}
	for i := 1; i < maxFailures; i++ {
		if f.failed("a", now) {
			t.Error("later failures are not reported again")
		}
	}
	if f.allowed("a", now) || !f.allowed("b", now) {
		t.Error("only the failing address is held")
	}
	if !f.allowed("a", now.Add(failureWindow+time.Second)) {
		t.Error("held for a window only")
	}
}

func TestStreamsFromTheConnectorAreClosed(t *testing.T) {
	a, b := net.Pipe()
	server, err := yamux.Server(a, (&Handler{}).muxConfig())
	if err != nil {
		t.Fatal(err)
	}
	go refuseStreams(server)
	client, err := yamux.Client(b, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	for i := 0; i < 50; i++ { // far past the backlog: none is left waiting
		st, err := client.OpenStream()
		if err != nil {
			t.Fatal(err)
		}
		_ = st.SetReadDeadline(time.Now().Add(2 * time.Second))
		if _, err := st.Read(make([]byte, 1)); err == nil {
			t.Fatal("a stream opened by the connector was served")
		}
		_ = st.Close()
	}
}
