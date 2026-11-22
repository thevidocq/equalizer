package server

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// The agent verifies the files of a draft with a HEAD preview: the verdict
// and the entry, without the body.
func TestTryAssetHead(t *testing.T) {
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/t/a.avif":
			w.Header().Set("Content-Type", "image/avif")
			_, _ = w.Write([]byte("\x00\x00\x00\x1cftypavif\x00\x00\x00\x00"))
		case "/t/b.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"))
		}
	}))
	defer legacy.Close()
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now()
	tr := &Try{Token: tryToken, Pub: pub, KeyID: "k1", Log: slog.New(slog.NewJSONHandler(io.Discard, nil)),
		now: func() time.Time { return now }}
	h := (&Admin{Try: tr}).Handler()
	doc := strings.Replace(tryMapDoc(legacy.URL), `"routes":[`, `"assets":[{"prefix":"/t/","types":["image/png"]}],"routes":[`, 1)
	m := signTry(t, key, doc, "try", now.Add(10*time.Minute))

	head := func(path string) tryResult {
		b, _ := json.Marshal(map[string]any{"map": m, "mode": "preview", "request": map[string]any{"method": "HEAD", "path": path}})
		req := httptest.NewRequest(http.MethodPost, "/maps/try", strings.NewReader(string(b)))
		req.Header.Set("Authorization", "Bearer "+tryToken)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var res tryResult
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil || rec.Code != 200 {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
		return res
	}
	res := head("/t/a.avif")
	if res.Refusal == nil || res.Refusal.Reason != "asset_type_not_allowed" || res.Entry == nil || *res.Entry != "asset:/t/" {
		t.Errorf("avif: %+v", res)
	}
	if res := head("/t/b.png"); res.Refusal != nil || !res.Accepted {
		t.Errorf("png: %+v", res)
	}
}
