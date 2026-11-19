package rebuild

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/textproto"
	"strings"
	"testing"
)

var png1x1 = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89")

func formWith(t *testing.T, name, ctype string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	_ = mw.WriteField("title", "notice")
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="file"; filename="`+name+`"`)
	h.Set("Content-Type", ctype)
	w, _ := mw.CreatePart(h)
	_, _ = w.Write(data)
	_ = mw.Close()
	return &b, mw.Boundary()
}

func TestRunnableFilesRefused(t *testing.T) {
	big := append(append(bytes.Repeat([]byte{0xff}, 40000), []byte("<?PhP system($_GET[c]); ?>")...), 0)
	refused := map[string][3]string{
		"script named as a picture": {"shell.php", "image/png", string(png1x1)},
		"double extension":          {"photo.php.png", "image/png", string(png1x1)},
		"trailing dot":              {"shell.php.", "image/png", string(png1x1)},
		"server settings":           {".htaccess", "text/plain", "AddType application/x-httpd-php .png"},
		"PHP inside a picture":      {"photo.png", "image/png", string(png1x1) + "<?php echo 1; ?>"},
		"PHP far into the file":     {"video.mp4", "video/mp4", string(big)},
		"page as a text file":       {"notes.txt", "text/plain", "<!DOCTYPE html><html><script>alert(1)</script>"},
		"not the picture it claims": {"photo.jpg", "image/jpeg", "just some text"},
		"program":                   {"setup.bin", "application/octet-stream", "MZ\x90\x00program"},
		"ASP page":                  {"report.aspx", "text/plain", "hello"},
	}
	for what, c := range refused {
		body, boundary := formWith(t, c[0], c[1], []byte(c[2]))
		var re *Error
		if _, err := Multipart(&bytes.Buffer{}, body, boundary, ""); !errors.As(err, &re) || re.Reason != ReasonFileRefused {
			t.Errorf("%s (%s): %v, want refused", what, c[0], err)
		}
	}
	allowed := map[string][3]string{
		"picture":           {"menu.png", "image/png", string(png1x1)},
		"text":              {"notes.txt", "text/plain", "Shopping list: milk, caffè."},
		"spreadsheet data":  {"data.csv", "text/csv", "a,b\n1,2\n"},
		"domain in a name":  {"www.acme.com.png", "image/png", string(png1x1)},
		"PDF":               {"report.pdf", "application/pdf", "%PDF-1.4\n%binary"},
		"no file chosen":    {"", "application/octet-stream", ""},
		"versioned name":    {"report.v2.final.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "PK\x03\x04zip"},
		"text about PHP is": {"intro.md", "text/markdown", "Use <? and ?> tags."},
	}
	for what, c := range allowed {
		body, boundary := formWith(t, c[0], c[1], []byte(c[2]))
		var out bytes.Buffer
		if _, err := Multipart(&out, body, boundary, ""); err != nil {
			t.Errorf("%s (%s): %v", what, c[0], err)
		} else if c[2] != "" && !strings.Contains(out.String(), c[2]) {
			t.Errorf("%s: content changed", what)
		}
	}
}
