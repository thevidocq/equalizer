// Package upload checks the files of a form before anything reaches the
// legacy application: size, real type (sniffed from the bytes, never the
// name or the browser's word) and, when the map asks for it, a rebuild of
// images from their pixels (content disarm: metadata and appended payloads
// are dropped). The legacy application gets a new random file name. Virus
// scanning is left to the security stack around the legacy application.
package upload

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/http"
	"slices"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
)

// Rejection reasons (field.rejected events).
const (
	ReasonTooLarge   = "file_too_large"
	ReasonType       = "file_type_not_allowed"
	ReasonUnreadable = "file_unreadable"
)

// Error is a refused file; it never carries the content.
type Error struct{ Field, Reason string }

func (e *Error) Error() string { return "file " + e.Field + " refused: " + e.Reason }

// File is a checked file, ready for the legacy request.
type File struct {
	Name string // new random name with the extension of the real type
	Type string // sniffed content type
	Data []byte
}

// CDRTypes are the types the gateway can rebuild.
var CDRTypes = []string{"image/png", "image/jpeg", "image/gif"}

var extensions = map[string]string{
	"image/png": ".png", "image/jpeg": ".jpg", "image/gif": ".gif", "image/webp": ".webp", "application/pdf": ".pdf",
	"text/plain": ".txt", "application/zip": ".zip",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   ".docx",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         ".xlsx",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": ".pptx",
	"application/vnd.oasis.opendocument.text":                                   ".odt",
	"application/vnd.oasis.opendocument.spreadsheet":                            ".ods",
}

// Check applies the field's rules to an uploaded file.
func Check(field string, spec mapmodel.FieldSpec, data []byte) (File, error) {
	if spec.MaxSizeKB > 0 && len(data) > spec.MaxSizeKB*1024 {
		return File{}, &Error{field, ReasonTooLarge}
	}
	t := Sniff(data)
	if !slices.Contains(spec.MIME, t) {
		return File{}, &Error{field, ReasonType}
	}
	if spec.Scan == "cdr" {
		clean, err := Rebuild(t, data)
		if err != nil {
			return File{}, &Error{field, ReasonUnreadable}
		}
		data = clean
	}
	return File{Name: randomName() + extensions[t], Type: t, Data: data}, nil
}

// Sniff returns the real type of the content.
func Sniff(data []byte) string {
	t := http.DetectContentType(data)
	if i := bytes.IndexByte([]byte(t), ';'); i > 0 {
		t = t[:i]
	}
	if t == "application/zip" {
		return office(data)
	}
	return t
}

// office tells OOXML and ODF documents from other zip files.
func office(data []byte) string {
	head := data[:min(len(data), 2048)]
	if len(data) >= 38 && bytes.HasPrefix(data[30:38], []byte("mimetype")) {
		for _, t := range []string{"application/vnd.oasis.opendocument.text", "application/vnd.oasis.opendocument.spreadsheet"} {
			if bytes.Contains(head, []byte(t)) {
				return t
			}
		}
	}
	if bytes.Contains(data, []byte("[Content_Types].xml")) {
		switch {
		case bytes.Contains(data, []byte("word/")):
			return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		case bytes.Contains(data, []byte("xl/")):
			return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		case bytes.Contains(data, []byte("ppt/")):
			return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
		}
	}
	return "application/zip"
}

// MaxPixels bounds the images rebuilt: a few KB of PNG can declare a
// picture that needs gigabytes once decoded.
const MaxPixels = 40_000_000

// Rebuild decodes an image and encodes its pixels again: whatever is not
// the picture (metadata, scripts, a second file glued after it) is lost.
func Rebuild(t string, data []byte) ([]byte, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > MaxPixels {
		return nil, errors.New("image too large once decoded")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	switch t {
	case "image/png":
		err = png.Encode(&out, img)
	case "image/jpeg":
		err = jpeg.Encode(&out, img, &jpeg.Options{Quality: 90})
	case "image/gif":
		err = gif.Encode(&out, img, nil)
	default:
		return nil, errors.New("this type cannot be rebuilt")
	}
	return out.Bytes(), err
}

func randomName() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
