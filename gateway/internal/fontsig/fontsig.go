// Package fontsig recognises web fonts by their signature, not by the media
// type the legacy server declares.
//
// Legacy servers often have no mapping for font files and send them as
// application/octet-stream, or with an old name such as
// application/x-font-woff. Rejecting them would break the rendering; trusting
// the declared type would let any file through. The signature decides.
package fontsig

import "bytes"

// Canonical media types.
const (
	WOFF  = "font/woff"
	WOFF2 = "font/woff2"
	TTF   = "font/ttf"
	OTF   = "font/otf"
	TTC   = "font/collection"
	EOT   = "application/vnd.ms-fontobject"
)

// aliases maps every name a server may declare to its canonical type.
var aliases = map[string]string{
	"font/woff":                     WOFF,
	"font/x-woff":                   WOFF,
	"application/font-woff":         WOFF,
	"application/x-font-woff":       WOFF,
	"font/woff2":                    WOFF2,
	"font/x-woff2":                  WOFF2,
	"application/font-woff2":        WOFF2,
	"application/x-font-woff2":      WOFF2,
	"font/ttf":                      TTF,
	"font/sfnt":                     TTF,
	"application/x-font-ttf":        TTF,
	"application/x-font-truetype":   TTF,
	"application/font-sfnt":         TTF,
	"font/otf":                      OTF,
	"application/x-font-otf":        OTF,
	"application/x-font-opentype":   OTF,
	"application/vnd.ms-fontobject": EOT,
	"font/collection":               TTC,
}

// generic are the types a server sends when it does not know the file.
var generic = map[string]bool{
	"application/octet-stream": true,
	"binary/octet-stream":      true,
	"application/unknown":      true,
	"text/plain":               true,
}

// Canonical returns the canonical font type for a declared media type, or "".
func Canonical(mediaType string) string { return aliases[mediaType] }

// Sniff returns the canonical font type read from the first bytes, or "".
func Sniff(body []byte) string {
	switch {
	case bytes.HasPrefix(body, []byte("wOFF")):
		return WOFF
	case bytes.HasPrefix(body, []byte("wOF2")):
		return WOFF2
	case bytes.HasPrefix(body, []byte{0, 1, 0, 0}), bytes.HasPrefix(body, []byte("true")):
		return TTF
	case bytes.HasPrefix(body, []byte("OTTO")):
		return OTF
	case bytes.HasPrefix(body, []byte("ttcf")):
		return TTC
	case eot(body):
		return EOT
	}
	return ""
}

// eot: an Embedded OpenType file has the magic number 0x504C at offset 34
// and one of the three known versions at offset 8 (little-endian).
func eot(body []byte) bool {
	if len(body) < 36 || body[34] != 0x4C || body[35] != 0x50 {
		return false
	}
	switch v := uint32(body[8]) | uint32(body[9])<<8 | uint32(body[10])<<16 | uint32(body[11])<<24; v {
	case 0x00010000, 0x00020001, 0x00020002:
		return true
	}
	return false
}

// Allowed reports whether a canonical font type is one the map's types allow.
func Allowed(font string, types []string) bool {
	for _, t := range types {
		if Same(Canonical(t), font) {
			return true
		}
	}
	return false
}

// Same reports whether two canonical types are the same font format, the
// sfnt ones (TrueType, OpenType, collections) counting as one: servers name
// them loosely (an OpenType file as font/ttf or application/font-sfnt).
func Same(a, b string) bool {
	sfnt := func(t string) bool { return t == TTF || t == OTF || t == TTC }
	return a == b || (sfnt(a) && sfnt(b))
}

// Resolve decides whether a legacy response is a font the map allows.
// declared is the media type the legacy server sent; allowed are the map's
// types. It returns the canonical type to serve, or "" if the response is not
// an allowed font. The signature must agree with the declared type, unless
// the declared type is generic.
func Resolve(declared string, body []byte, allowed []string) string {
	sniffed := Sniff(body)
	if sniffed == "" {
		return ""
	}
	if c := Canonical(declared); !Same(c, sniffed) && !generic[declared] {
		return ""
	}
	if Allowed(sniffed, allowed) {
		return sniffed
	}
	return ""
}
