package upload

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
)

func pngBytes(t *testing.T) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(1, 1, color.RGBA{255, 0, 0, 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestCheck(t *testing.T) {
	spec := mapmodel.FieldSpec{Type: "file", MIME: []string{"image/png", "application/pdf"}, MaxSizeKB: 64}
	img := pngBytes(t)
	f, err := Check("doc", spec, img)
	if err != nil || f.Type != "image/png" || !strings.HasSuffix(f.Name, ".png") || len(f.Name) != 28 {
		t.Fatalf("png: %+v %v", f, err)
	}
	cases := map[string]struct {
		data   []byte
		reason string
	}{
		"html renamed as png": {[]byte("<html><script>alert(1)</script>"), ReasonType},
		"too large":           {append([]byte("%PDF-1.4\n"), make([]byte, 70*1024)...), ReasonTooLarge},
	}
	for name, c := range cases {
		_, err := Check("doc", spec, c.data)
		var ue *Error
		if !errors.As(err, &ue) || ue.Reason != c.reason {
			t.Errorf("%s: %v, want %s", name, err, c.reason)
		}
	}
	// cdr: an image with a second file glued after it comes out clean.
	cdr := spec
	cdr.Scan = "cdr"
	polyglot := append(append([]byte{}, img...), []byte("<?php system($_GET['c']); ?>")...)
	f, err = Check("doc", cdr, polyglot)
	if err != nil || bytes.Contains(f.Data, []byte("<?php")) {
		t.Fatalf("rebuild: %v, payload kept: %v", err, bytes.Contains(f.Data, []byte("<?php")))
	}
}

func TestSniffOffice(t *testing.T) {
	docx := append([]byte("PK\x03\x04"), []byte("....[Content_Types].xml....word/document.xml")...)
	if got := Sniff(docx); got != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
		t.Fatalf("docx: %s", got)
	}
}
