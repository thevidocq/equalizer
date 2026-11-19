package server

import (
	"bytes"
	"crypto/sha256"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
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
