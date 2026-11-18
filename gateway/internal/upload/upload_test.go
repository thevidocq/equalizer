package upload

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"net"
	"strings"
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
)

type fakeScanner struct {
	infected bool
	err      error
}

func (f fakeScanner) Scan(context.Context, []byte) error {
	if f.infected {
		return ErrInfected
	}
	return f.err
}

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
	spec := mapmodel.FieldSpec{Type: "file", MIME: []string{"image/png", "application/pdf"}, MaxSizeKB: 64, Scan: "av"}
	ctx := context.Background()
	img := pngBytes(t)
	f, err := Check(ctx, "doc", spec, img, fakeScanner{})
	if err != nil || f.Type != "image/png" || !strings.HasSuffix(f.Name, ".png") || len(f.Name) != 28 {
		t.Fatalf("png: %+v %v", f, err)
	}
	cases := map[string]struct {
		data    []byte
		scanner Scanner
		reason  string
	}{
		"html renamed as png": {[]byte("<html><script>alert(1)</script>"), fakeScanner{}, ReasonType},
		"too large":           {append([]byte("%PDF-1.4\n"), make([]byte, 70*1024)...), fakeScanner{}, ReasonTooLarge},
		"infected":            {[]byte("%PDF-1.4\n%%EOF"), fakeScanner{infected: true}, ReasonInfected},
		"scanner down":        {[]byte("%PDF-1.4\n%%EOF"), fakeScanner{err: errors.New("down")}, ReasonScan},
		"no scanner":          {[]byte("%PDF-1.4\n%%EOF"), nil, ReasonScan},
	}
	for name, c := range cases {
		_, err := Check(ctx, "doc", spec, c.data, c.scanner)
		var ue *Error
		if !errors.As(err, &ue) || ue.Reason != c.reason {
			t.Errorf("%s: %v, want %s", name, err, c.reason)
		}
	}
	// av+cdr: an image with a second file glued after it comes out clean.
	cdr := spec
	cdr.Scan = "av+cdr"
	polyglot := append(append([]byte{}, img...), []byte("<?php system($_GET['c']); ?>")...)
	f, err = Check(ctx, "doc", cdr, polyglot, fakeScanner{})
	if err != nil || bytes.Contains(f.Data, []byte("<?php")) {
		t.Fatalf("rebuild: %v, payload kept: %v", err, bytes.Contains(f.Data, []byte("<?php")))
	}
}

func TestClamd(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				cmd := make([]byte, 10)
				_, _ = io.ReadFull(c, cmd)
				var all []byte
				for {
					var n uint32
					if binary.Read(c, binary.BigEndian, &n) != nil || n == 0 {
						break
					}
					b := make([]byte, n)
					_, _ = io.ReadFull(c, b)
					all = append(all, b...)
				}
				if bytes.Contains(all, []byte("EICAR")) {
					_, _ = c.Write([]byte("stream: Eicar-Test-Signature FOUND\x00"))
				} else {
					_, _ = c.Write([]byte("stream: OK\x00"))
				}
			}(c)
		}
	}()
	s := &Clamd{Addr: ln.Addr().String()}
	if err := s.Scan(context.Background(), bytes.Repeat([]byte("a"), 200<<10)); err != nil {
		t.Fatalf("clean: %v", err)
	}
	if err := s.Scan(context.Background(), []byte("X5O!P%@AP EICAR")); !errors.Is(err, ErrInfected) {
		t.Fatalf("infected: %v", err)
	}
	if err := (&Clamd{Addr: "127.0.0.1:1"}).Scan(context.Background(), []byte("x")); err == nil {
		t.Fatal("unreachable clamd accepted")
	}
}

func TestSniffOffice(t *testing.T) {
	docx := append([]byte("PK\x03\x04"), []byte("....[Content_Types].xml....word/document.xml")...)
	if got := Sniff(docx); got != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
		t.Fatalf("docx: %s", got)
	}
}
