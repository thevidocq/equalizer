package server

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
)

// mediaSite serves /media/ with video/mp4 allowed, max_size_kb and the legacy
// budget given.
func mediaSite(t *testing.T, legacy http.Handler, maxKB, concurrency int) http.Handler {
	t.Helper()
	st := rebuild(t, testSite(t, legacy), func(m *mapmodel.Model) {
		m.Assets = append(m.Assets, mapmodel.Asset{Prefix: "/media/", Types: []string{"video/mp4"}, MaxSizeKB: maxKB})
		if concurrency > 0 {
			m.Defaults.LegacyBudget.MaxConcurrency = concurrency
		}
	})
	return newPublic(t, st, io.Discard)
}

var mp4Head = []byte("\x00\x00\x00\x18ftypmp42")

// Paused players must not hold the site's legacy budget: with two slots and
// two videos nobody reads, the home page still answers.
func TestMediaPausedPlayersKeepTheSiteUp(t *testing.T) {
	big := append(append([]byte{}, mp4Head...), bytes.Repeat([]byte{7}, 64<<20)...)
	h := mediaSite(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/media/v.mp4" {
			w.Header().Set("Content-Type", "video/mp4")
			http.ServeContent(w, r, "v.mp4", time.Time{}, bytes.NewReader(big))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body>home</body></html>"))
	}), 0, 2)
	srv := httptest.NewServer(h)
	defer srv.Close()
	for i := 0; i < 2; i++ { // two players that never read
		c, err := net.Dial("tcp", srv.Listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		fmt.Fprintf(c, "GET /media/v.mp4 HTTP/1.1\r\nHost: %s\r\n\r\n", srv.Listener.Addr())
		if line, err := bufio.NewReader(c).ReadString('\n'); err != nil || !strings.Contains(line, "200") {
			t.Fatalf("video: %q %v", line, err)
		}
	}
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("home page while two videos are paused: %d", resp.StatusCode)
	}
}

// The range a client chooses never skips the signature: a "video" that is a
// page is refused whatever range is asked; a real video still seeks.
func TestMediaRangeKeepsTheSignatureCheck(t *testing.T) {
	evil := []byte("<html><script>alert(document.domain)</script></html>")
	good := append(append([]byte{}, mp4Head...), bytes.Repeat([]byte{7}, 4096)...)
	h := mediaSite(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		switch r.URL.Path {
		case "/media/evil.mp4":
			http.ServeContent(w, r, "evil.mp4", time.Time{}, bytes.NewReader(evil))
		case "/media/good.mp4":
			http.ServeContent(w, r, "good.mp4", time.Time{}, bytes.NewReader(good))
		case "/media/unasked.mp4": // a 206 nobody asked for
			w.Header().Set("Content-Range", "bytes 0-11/12")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(mp4Head)
		}
	}), 0, 0)
	for _, rng := range []string{"bytes=1-", "bytes=10-20", "bytes=-5"} {
		if rec := get(h, "GET", "/media/evil.mp4", http.Header{"Range": {rng}}); rec.Code != http.StatusNotFound {
			t.Errorf("evil with %s: %d %q", rng, rec.Code, rec.Body.String())
		}
	}
	rec := get(h, "GET", "/media/good.mp4", http.Header{"Range": {"bytes=1000-1099"}})
	if rec.Code != http.StatusPartialContent || rec.Body.Len() != 100 {
		t.Errorf("good video seek: %d, %d bytes", rec.Code, rec.Body.Len())
	}
	if rec := get(h, "GET", "/media/unasked.mp4", nil); rec.Code != http.StatusNotFound {
		t.Errorf("206 nobody asked for: %d", rec.Code)
	}
}

// A file the map would not serve gives no size through a 416.
func TestMedia416OnlyForServedFiles(t *testing.T) {
	h := mediaSite(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/media/backup.mp4" {
			w.Header().Set("Content-Type", "application/pdf")
		} else {
			w.Header().Set("Content-Type", "video/mp4")
		}
		http.ServeContent(w, r, "x", time.Time{}, bytes.NewReader(append(append([]byte{}, mp4Head...), make([]byte, 100)...)))
	}), 0, 0)
	rec := get(h, "GET", "/media/backup.mp4", http.Header{"Range": {"bytes=999999-"}})
	if rec.Code != http.StatusNotFound || rec.Header().Get("Content-Range") != "" {
		t.Errorf("416 of a file not served: %d, Content-Range %q", rec.Code, rec.Header().Get("Content-Range"))
	}
	rec = get(h, "GET", "/media/ok.mp4", http.Header{"Range": {"bytes=999999-"}})
	if rec.Code != http.StatusRequestedRangeNotSatisfiable || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("416 of a served video: %d, nosniff %q", rec.Code, rec.Header().Get("X-Content-Type-Options"))
	}
}

// A body longer than max_size_kb that the legacy server did not announce is
// aborted, never served short as if whole.
func TestMediaUnannouncedSizeAborted(t *testing.T) {
	h := mediaSite(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write(mp4Head)
		w.(http.Flusher).Flush() // chunked: no Content-Length
		_, _ = w.Write(make([]byte, 4096))
	}), 1, 0)
	srv := httptest.NewServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/media/long.mp4")
	if err != nil {
		return // aborted before the headers left: a failure, as wanted
	}
	defer resp.Body.Close()
	n, err := io.Copy(io.Discard, resp.Body)
	if err == nil {
		t.Fatalf("a %d-byte body over a 1 KiB limit ended as if complete", n)
	}
}
