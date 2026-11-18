package server

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/clientip"
	"github.com/thevidocq/equalizer/gateway/internal/formtoken"
	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/site"
	"github.com/thevidocq/equalizer/gateway/internal/telemetry"
	"github.com/thevidocq/equalizer/gateway/internal/upload"
)

type cleanScanner struct{ calls int }

func (c *cleanScanner) Scan(context.Context, []byte) error { c.calls++; return nil }

func TestUpload(t *testing.T) {
	var got struct {
		name, ctype, title string
		data               []byte
	}
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, `<form action="/upload.do" method="post" enctype="multipart/form-data">
<input name="title"><input type="file" name="attachment"></form>`)
			return
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		f, h, err := r.FormFile("attachment")
		if err == nil {
			got.data, _ = io.ReadAll(f)
			got.name, got.ctype = h.Filename, h.Header.Get("Content-Type")
		}
		got.title = r.FormValue("title")
		_, _ = io.WriteString(w, "<p>received</p>")
	}))
	defer legacy.Close()
	u, _ := url.Parse(legacy.URL)
	no := false
	m := &mapmodel.Model{
		Meta:   mapmodel.Meta{Tenant: "t", App: "a", Version: 1, Status: "published"},
		Legacy: mapmodel.Legacy{BaseURL: legacy.URL, Reach: mapmodel.Reach{Mode: "direct", Allow: []string{u.Host}}},
		Roles:  mapmodel.Roles{Anonymous: "guest", List: map[string]any{"guest": nil}},
		Routes: []mapmodel.Route{{ID: "page", Path: "/send", Method: "GET", Roles: []string{"guest"},
			LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/form.jsp"}}},
		Forms: []mapmodel.Form{{ID: "upload", Route: "/send/upload", Roles: []string{"guest"},
			Fields: map[string]mapmodel.FieldSpec{
				"title":      {Type: "string", MaxLength: 50},
				"attachment": {Type: "file", MIME: []string{"image/png"}, MaxSizeKB: 64, Scan: "av+cdr", Required: &no},
			},
			LegacyRequest: mapmodel.LegacyRequest{Method: "POST", Path: "/upload.do",
				Form: map[string]string{"title": "{{fields.title}}"}, Files: map[string]string{"attachment": "{{fields.attachment}}"}}}},
	}
	scanner := &cleanScanner{}
	st, err := site.BuildWith(m, site.Options{Scanner: scanner})
	if err != nil {
		t.Fatal(err)
	}
	sealer, _ := formtoken.New(formtoken.RandomKey())
	h := (&Public{Maps: fixedMaps{st}, IPs: clientip.New(nil), Forms: sealer, Scanner: scanner,
		Log: slog.New(slog.NewJSONHandler(io.Discard, nil)), Events: telemetry.NewEmitter(io.Discard, "test")}).Handler()

	page := get(h, "GET", "/send", nil)
	token := tokenRe.FindStringSubmatch(page.Body.String())
	if token == nil {
		t.Fatalf("form not bound: %s", page.Body.String())
	}
	cookie := page.Result().Cookies()[0]
	img := image.NewRGBA(image.Rect(0, 0, 3, 3))
	var pic bytes.Buffer
	_ = png.Encode(&pic, img)

	post := func(file []byte, fname string, extra func(*multipart.Writer)) *httptest.ResponseRecorder {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		_ = mw.WriteField(formtoken.Field, token[1])
		_ = mw.WriteField("title", "Shed")
		if file != nil {
			fw, _ := mw.CreateFormFile("attachment", fname)
			_, _ = fw.Write(file)
		}
		if extra != nil {
			extra(mw)
		}
		_ = mw.Close()
		req := httptest.NewRequest("POST", "/send/upload", &body)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	polyglot := append(pic.Bytes(), []byte("<?php evil(); ?>")...)
	if rec := post(polyglot, "../../etc/shell.php", nil); rec.Code != 200 {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	if got.title != "Shed" || got.ctype != "image/png" || !strings.HasSuffix(got.name, ".png") || strings.Contains(got.name, "shell") ||
		bytes.Contains(got.data, []byte("<?php")) || scanner.calls != 1 {
		t.Fatalf("legacy got %q %q %q payload kept=%v scans=%d", got.title, got.name, got.ctype, bytes.Contains(got.data, []byte("<?php")), scanner.calls)
	}
	if rec := post([]byte("<html><script>x</script>"), "a.png", nil); rec.Code != 400 {
		t.Errorf("html as png: %d", rec.Code)
	}
	if rec := post(pic.Bytes(), "a.png", func(mw *multipart.Writer) {
		fw, _ := mw.CreateFormFile("other", "x.png")
		_, _ = fw.Write(pic.Bytes())
	}); rec.Code != 400 {
		t.Errorf("undeclared file field: %d", rec.Code)
	}
	if rec := post(nil, "", nil); rec.Code != 200 {
		t.Errorf("optional file absent: %d", rec.Code)
	}

	// Without a scanner the form is not served.
	st2, _ := site.BuildWith(m, site.Options{})
	if st2.FormUnsupported("upload") == "" {
		t.Error("a form with files served without a scanner")
	}
	var _ upload.Scanner = scanner
}
