// Package charset detects the encoding of legacy pages and converts them to
// UTF-8, so faithful rendering does not alter accented letters.
//
// Supported encodings: UTF-8, ISO-8859-1, ISO-8859-15, Windows-1252 (the
// most common in old Western European sites). Others are rejected: an error
// page is better than a page with the wrong characters.
package charset

import (
	"bytes"
	"errors"
	"fmt"
	"mime"
	"regexp"
	"strings"
	"unicode/utf8"
)

// ErrUnsupported reports an unhandled encoding.
var ErrUnsupported = errors.New("unsupported encoding")

// Canonical names returned by Detect.
const (
	UTF8        = "utf-8"
	ISO88591    = "iso-8859-1"
	ISO885915   = "iso-8859-15"
	Windows1252 = "windows-1252"
)

var aliases = map[string]string{
	"utf-8": UTF8, "utf8": UTF8,
	"iso-8859-1": ISO88591, "iso8859-1": ISO88591, "latin1": ISO88591, "latin-1": ISO88591, "l1": ISO88591,
	"iso-8859-15": ISO885915, "iso8859-15": ISO885915, "latin9": ISO885915, "latin-9": ISO885915,
	"windows-1252": Windows1252, "cp1252": Windows1252, "x-cp1252": Windows1252,
	"us-ascii": Windows1252, "ascii": Windows1252,
}

var metaRe = regexp.MustCompile(`(?i)<meta[^>]+charset\s*=\s*["']?\s*([a-z0-9_\-:.]+)`)

// Detect chooses the encoding: Content-Type header, then <meta> in the first
// 1024 bytes, then UTF-8 if the bytes are valid UTF-8, otherwise
// Windows-1252 (the browsers' default for Western pages).
func Detect(contentType string, body []byte) (string, error) {
	if _, params, err := mime.ParseMediaType(contentType); err == nil {
		if cs := params["charset"]; cs != "" {
			return normalize(cs)
		}
	}
	head := body
	if len(head) > 1024 {
		head = head[:1024]
	}
	if m := metaRe.FindSubmatch(head); m != nil {
		return normalize(string(m[1]))
	}
	if utf8.Valid(body) {
		return UTF8, nil
	}
	return Windows1252, nil
}

func normalize(name string) (string, error) {
	if cs, ok := aliases[strings.ToLower(strings.TrimSpace(name))]; ok {
		return cs, nil
	}
	return "", fmt.Errorf("%w: %q", ErrUnsupported, name)
}

// ToUTF8 converts body from the given encoding to UTF-8.
func ToUTF8(name string, body []byte) ([]byte, error) {
	switch name {
	case UTF8:
		if utf8.Valid(body) {
			return body, nil
		}
		return bytes.ToValidUTF8(body, []byte("�")), nil
	case ISO88591:
		return decodeSingleByte(body, nil), nil
	case ISO885915:
		return decodeSingleByte(body, iso885915), nil
	case Windows1252:
		return decodeSingleByte(body, windows1252), nil
	}
	return nil, fmt.Errorf("%w: %q", ErrUnsupported, name)
}

// decodeSingleByte decodes a single-byte encoding: bytes not listed in
// overrides map to their Latin-1 code point.
func decodeSingleByte(body []byte, overrides map[byte]rune) []byte {
	out := make([]byte, 0, len(body)+len(body)/8)
	for _, b := range body {
		if b < 0x80 {
			out = append(out, b)
			continue
		}
		r := rune(b)
		if o, ok := overrides[b]; ok {
			r = o
		}
		out = utf8.AppendRune(out, r)
	}
	return out
}

// windows1252 differs from ISO-8859-1 in the 0x80–0x9F range.
var windows1252 = map[byte]rune{
	0x80: '€', 0x82: '‚', 0x83: 'ƒ', 0x84: '„', 0x85: '…', 0x86: '†', 0x87: '‡',
	0x88: 'ˆ', 0x89: '‰', 0x8A: 'Š', 0x8B: '‹', 0x8C: 'Œ', 0x8E: 'Ž',
	0x91: '‘', 0x92: '’', 0x93: '“', 0x94: '”', 0x95: '•', 0x96: '–', 0x97: '—',
	0x98: '˜', 0x99: '™', 0x9A: 'š', 0x9B: '›', 0x9C: 'œ', 0x9E: 'ž', 0x9F: 'Ÿ',
}

// iso885915 differs from ISO-8859-1 in eight positions.
var iso885915 = map[byte]rune{
	0xA4: '€', 0xA6: 'Š', 0xA8: 'š', 0xB4: 'Ž', 0xB8: 'ž', 0xBC: 'Œ', 0xBD: 'œ', 0xBE: 'Ÿ',
}
