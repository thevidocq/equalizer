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
