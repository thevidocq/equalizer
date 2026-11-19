package server

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/useragent"
)

func TestLargeUploadStreamed(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	var got [32]byte
	var title string
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		b, _ := io.ReadAll(f)
		got, title = sha256.Sum256(b), r.FormValue("title")
		_, _ = io.WriteString(w, "stored")
	}))
	defer legacy.Close()
	h := areaPublic(t, legacy.URL, 4096).Handler()
	send := func(size int) int {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		_ = mw.WriteField("title", "report")
		fw, _ := mw.CreateFormFile("file", `C:\\Users\\me\\report.pdf`)
		_, _ = fw.Write(bytes.Repeat([]byte{0xAB}, size))
		_ = mw.Close()
		req := httptest.NewRequest("POST", "/wp-admin/async-upload.php", &buf)
		req.Host = "blog.example"
		req.Header.Set("Content-Type", mw.FormDataContentType())
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	if code := send(3 << 20); code != 200 || got != sha256.Sum256(bytes.Repeat([]byte{0xAB}, 3<<20)) || title != "report" {
		t.Fatalf("upload within the limit: %d %q", code, title)
	}
	if code := send(5 << 20); code != http.StatusRequestEntityTooLarge {
		t.Errorf("upload over the limit: %d", code)
	}
	left, _ := filepath.Glob(filepath.Join(os.Getenv("TMPDIR"), "eq-upload-*"))
	if len(left) != 0 {
		t.Errorf("temporary files left: %v", left)
	}
}

func TestLegacyCompressionAndUserAgent(t *testing.T) {
	var ua, ae string
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua, ae = r.UserAgent(), r.Header.Get("Accept-Encoding")
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Content-Encoding", "gzip") // some apps compress whatever was asked
		zw := gzip.NewWriter(w)
		_, _ = io.WriteString(zw, "<html><body><p>Ciao</p></body></html>")
		_ = zw.Close()
	}))
	defer legacy.Close()
	h := areaPublic(t, legacy.URL, 0).Handler()
	req := httptest.NewRequest("GET", "/mobile", nil)
	req.Host = "blog.example"
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) Safari/604.1' OR '1'='1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte("<p>Ciao</p>")) || rec.Header().Get("Content-Encoding") != "" {
		t.Fatalf("compressed page: %d %q %q", rec.Code, rec.Header().Get("Content-Encoding"), rec.Body.String())
	}
	if ua != useragent.SafariIPhone || ae != "identity" {
		t.Errorf("towards the legacy app: UA %q, Accept-Encoding %q", ua, ae)
	}
}

func TestRawUpload(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	var gotType string
	var gotLen int
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotType, gotLen = r.Header.Get("Content-Type"), len(b)
		_, _ = io.WriteString(w, "ok")
	}))
	defer legacy.Close()
	h := areaPublic(t, legacy.URL, 4096).Handler()
	send := func(ctype string, body []byte) int {
		req := httptest.NewRequest("POST", "/upload/chunk", bytes.NewReader(body))
		req.Host = "blog.example"
		req.Header.Set("Content-Type", ctype)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 2<<20)...)
	if code := send("image/jpeg", png); code != 200 || gotType != "image/png" || gotLen != len(png) {
		t.Fatalf("picture: %d %q %d", code, gotType, gotLen)
	}
	if code := send("image/png", []byte("<html><script>alert(1)</script></html>")); code != http.StatusUnsupportedMediaType {
		t.Errorf("page as a picture: %d", code)
	}
	if code := send("application/octet-stream", []byte("MZ\x90\x00program")); code != http.StatusUnsupportedMediaType {
		t.Errorf("program: %d", code)
	}
	if code := send("application/octet-stream", bytes.Repeat([]byte{1}, 5<<20)); code != http.StatusRequestEntityTooLarge {
		t.Errorf("over the limit: %d", code)
	}
	left, _ := filepath.Glob(filepath.Join(os.Getenv("TMPDIR"), "eq-upload-*"))
	if len(left) != 0 {
		t.Errorf("temporary files left: %v", left)
	}
}
