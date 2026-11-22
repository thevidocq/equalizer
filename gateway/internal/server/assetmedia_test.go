package server

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
)

func TestMediaAssets(t *testing.T) {
	// Larger than the 16 MiB a buffered response may hold.
	video := append([]byte("\x00\x00\x00\x18ftypmp42"), bytes.Repeat([]byte{7}, 17<<20)...)
	var ranges []string
	st := testSite(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ranges = append(ranges, r.Header.Get("Range"))
		switch r.URL.Path {
		case "/media/intro.mp4":
			w.Header().Set("Content-Type", "video/mp4")
			http.ServeContent(w, r, "intro.mp4", time.Time{}, bytes.NewReader(video))
		case "/media/fake.mp4":
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write([]byte("<script>alert(1)</script>"))
		case "/media/page.mp4":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<html></html>"))
		}
	}))
	st = rebuild(t, st, func(m *mapmodel.Model) {
		m.Assets = append(m.Assets, mapmodel.Asset{Prefix: "/media/", Types: []string{"video/mp4", "text/html"}})
	})
	h := newPublic(t, st, io.Discard)

	rec := get(h, "GET", "/media/intro.mp4", nil)
	if rec.Code != 200 || rec.Body.Len() != len(video) || rec.Header().Get("Accept-Ranges") != "bytes" {
		t.Fatalf("full video: %d, %d bytes, Accept-Ranges %q", rec.Code, rec.Body.Len(), rec.Header().Get("Accept-Ranges"))
	}
	rec = get(h, "GET", "/media/intro.mp4", http.Header{"Range": {"bytes=1000-1999"}})
	if rec.Code != http.StatusPartialContent || rec.Body.Len() != 1000 ||
		!strings.HasPrefix(rec.Header().Get("Content-Range"), "bytes 1000-1999/") {
		t.Errorf("range: %d, %d bytes, Content-Range %q", rec.Code, rec.Body.Len(), rec.Header().Get("Content-Range"))
	}
	ranges = nil
	rec = get(h, "GET", "/media/intro.mp4", http.Header{"Range": {"bytes=0-1,5-9"}})
	if rec.Code != 200 || ranges[0] != "" {
		t.Errorf("a range list must not reach the legacy application: %d, Range %q", rec.Code, ranges[0])
	}
	for _, target := range []string{"/media/fake.mp4", "/media/page.mp4"} {
		if rec := get(h, "GET", target, nil); rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", target, rec.Code)
		}
	}
}
