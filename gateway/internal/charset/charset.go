// Package charset riconosce la codifica delle pagine del legacy e le
// converte in UTF-8, così la resa fedele non altera le lettere accentate.
//
// Codifiche supportate: UTF-8, ISO-8859-1, ISO-8859-15, Windows-1252
// (le più diffuse nei siti italiani datati). Le altre vengono rifiutate:
// meglio una pagina d'errore che una pagina con i caratteri sbagliati.
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

// ErrUnsupported indica una codifica non gestita.
var ErrUnsupported = errors.New("codifica non supportata")

// Nomi canonici restituiti da Detect.
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

// Detect sceglie la codifica: header Content-Type, poi <meta> nei primi
// 1024 byte, poi UTF-8 se i byte sono UTF-8 valido, altrimenti Windows-1252
// (il comportamento predefinito dei browser per le pagine occidentali).
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

// ToUTF8 converte body dalla codifica indicata a UTF-8.
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

// decodeSingleByte decodifica una codifica a un byte: i byte non presenti
// in overrides valgono il loro code point Latin-1.
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

// windows1252 differisce da ISO-8859-1 nell'intervallo 0x80–0x9F.
var windows1252 = map[byte]rune{
	0x80: '€', 0x82: '‚', 0x83: 'ƒ', 0x84: '„', 0x85: '…', 0x86: '†', 0x87: '‡',
	0x88: 'ˆ', 0x89: '‰', 0x8A: 'Š', 0x8B: '‹', 0x8C: 'Œ', 0x8E: 'Ž',
	0x91: '‘', 0x92: '’', 0x93: '“', 0x94: '”', 0x95: '•', 0x96: '–', 0x97: '—',
	0x98: '˜', 0x99: '™', 0x9A: 'š', 0x9B: '›', 0x9C: 'œ', 0x9E: 'ž', 0x9F: 'Ÿ',
}

// iso885915 differisce da ISO-8859-1 in otto posizioni.
var iso885915 = map[byte]rune{
	0xA4: '€', 0xA6: 'Š', 0xA8: 'š', 0xB4: 'Ž', 0xB8: 'ž', 0xBC: 'Œ', 0xBD: 'œ', 0xBE: 'Ÿ',
}
