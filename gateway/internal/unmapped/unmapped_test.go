package unmapped

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReporter(t *testing.T) {
	var got struct{ Items []item }
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	r := &Reporter{URL: srv.URL, Token: "tok", Client: srv.Client()}
	for _, ip := range []string{"1.1.1.1", "1.1.1.1", "2.2.2.2"} {
		r.Record("blog", "GET", "/events/", ip)
	}
	r.Record("blog", "GET", "/"+strings.Repeat("a", 2000), "3.3.3.3")
	r.Record("blog", "GET", "no-slash", "3.3.3.3")
	if err := r.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer tok" || len(got.Items) != 2 {
		t.Fatalf("sent: %q %+v", auth, got.Items)
	}
	for _, it := range got.Items {
		if it.Path == "/events/" && (it.Hits != 3 || it.Clients != 2) {
			t.Errorf("counts: %+v", it)
		}
		if len(it.Path) > MaxPath {
			t.Errorf("path not truncated: %d", len(it.Path))
		}
	}
	for i := 0; i < MaxEntries+50; i++ {
		r.Record("blog", "GET", "/p"+strings.Repeat("x", i%7)+string(rune('a'+i%26))+itoa(i), "1.1.1.1")
	}
	if n := len(r.take()); n != MaxEntries {
		t.Fatalf("entries not bounded: %d", n)
	}
	var nilR *Reporter
	nilR.Record("a", "GET", "/", "x")
	_ = nilR.Flush(context.Background())
}

func itoa(i int) string {
	b := []byte{}
	for i > 0 || len(b) == 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

// A visitor is sent as a keyed hash: the same address gives the same key
// across flushes (so the panel counts it once), never the address, and the
// key comes from the gateway's own secret, not from the token it shares.
func TestVisitorKeys(t *testing.T) {
	var sent []item
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b struct{ Items []item }
		_ = json.NewDecoder(r.Body).Decode(&b)
		sent = append(sent, b.Items...)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	r := &Reporter{URL: srv.URL, Token: "tok", Key: []byte("gateway-only-secret"), Client: srv.Client()}
	r.Record("blog", "GET", "/a.pdf", "203.0.113.7")
	_ = r.Flush(context.Background())
	r.Record("blog", "GET", "/a.pdf", "203.0.113.7")
	_ = r.Flush(context.Background())
	if len(sent) != 2 || len(sent[0].ClientKeys) != 1 || sent[0].ClientKeys[0] != sent[1].ClientKeys[0] {
		t.Fatalf("keys: %+v", sent)
	}
	if k := sent[0].ClientKeys[0]; len(k) != 16 || strings.Contains(k, "203") {
		t.Errorf("key %q", k)
	}
	// the panel's token alone cannot recompute a key: the secret is the gateway's
	sameToken := &Reporter{Token: "tok", Key: []byte("another-secret")}
	if a, b := r.visitor("/a.pdf", "203.0.113.7"), sameToken.visitor("/a.pdf", "203.0.113.7"); a == b {
		t.Error("the key depends on the token, not on the gateway's secret")
	}
	random := &Reporter{Token: "tok"}
	if a, b := r.visitor("/a.pdf", "203.0.113.7"), random.visitor("/a.pdf", "203.0.113.7"); a == b || random.visitor("/a.pdf", "203.0.113.7") != b {
		t.Error("without a key: a random one, stable in the process")
	}
	if r.visitor("/a.pdf", "203.0.113.7") == r.visitor("/b.pdf", "203.0.113.7") {
		t.Error("one visitor's key links its requests across paths")
	}
	for i := 0; i < 40; i++ {
		r.Record("blog", "GET", "/b.pdf", "198.51.100."+itoa(i))
	}
	if it := r.take(); len(it[0].ClientKeys) != MaxSentKeys || it[0].Clients != 40 {
		t.Errorf("sent keys not bounded: %d", len(it[0].ClientKeys))
	}
}
