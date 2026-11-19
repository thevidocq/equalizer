package upload

import (
	"bytes"
	"errors"
	"net/http"
	"strings"
)

// Raw bodies: some uploaders send a file as the whole request body
// (Content-Type image/jpeg, application/octet-stream…) instead of a form.

// ErrRawType is a raw body whose bytes are not what it claims, or are a
// program or a page.
var ErrRawType = errors.New("raw body refused: type")

var rawTypes = map[string]bool{
	"application/pdf": true, "application/octet-stream": true, "application/zip": true, "application/x-zip-compressed": true,
	"application/msword": true, "application/vnd.ms-excel": true, "application/vnd.ms-powerpoint": true,
	"application/ogg": true,
}

// IsRawType reports whether a declared media type is a file sent as the body.
func IsRawType(mt string) bool {
	if rawTypes[mt] || strings.HasPrefix(mt, "application/vnd.openxmlformats-officedocument.") ||
		strings.HasPrefix(mt, "application/vnd.oasis.opendocument.") {
		return true
	}
	for _, p := range []string{"image/", "audio/", "video/"} {
		if strings.HasPrefix(mt, p) {
			return mt != "image/svg+xml" // SVG is a document with scripts, not a picture
		}
	}
	return false
}

// programs are the starts of executables and scripts: never a file upload.
var programs = [][]byte{
	[]byte("MZ"), []byte("\x7fELF"), []byte("#!"), []byte("\xca\xfe\xba\xbe"), []byte("\xcf\xfa\xed\xfe"),
	[]byte("\xce\xfa\xed\xfe"), []byte("<?php"), []byte("<%"),
}

// CheckRaw compares the declared type of a raw body with its first bytes
// and returns the type to send on. Pages, scripts and programs are refused
// whatever they claim.
func CheckRaw(declared string, head []byte) (string, error) {
	for _, p := range programs {
		if bytes.HasPrefix(head, p) {
			return "", ErrRawType
		}
	}
	got := http.DetectContentType(head)
	if i := strings.IndexByte(got, ';'); i > 0 {
		got = got[:i]
	}
	if strings.HasPrefix(got, "text/") || strings.Contains(got, "xml") || strings.Contains(got, "html") {
		return "", ErrRawType
	}
	family := func(t string) string { return t[:strings.IndexByte(t, '/')] }
	switch {
	case declared == "application/octet-stream":
		return declared, nil
	case got == "application/octet-stream":
		// Not recognised from the first bytes (many video, audio and office
		// formats): allowed, never as a picture or a PDF.
		if strings.HasPrefix(declared, "image/") || declared == "application/pdf" {
			return "", ErrRawType
		}
		return declared, nil
	case strings.HasPrefix(declared, "image/") || strings.HasPrefix(declared, "audio/") || strings.HasPrefix(declared, "video/"):
		if family(got) != family(declared) {
			return "", ErrRawType
		}
		return got, nil
	case declared == "application/pdf":
		if got != "application/pdf" {
			return "", ErrRawType
		}
		return got, nil
	case got == "application/zip": // office documents and archives are zip files
		return declared, nil
	}
	return "", ErrRawType
}
