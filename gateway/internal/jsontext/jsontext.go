// Package jsontext checks the text of a JSON document before it is decoded.
// Go's decoder replaces invalid UTF-8 and lone surrogate escapes ("\ud800")
// with U+FFFD instead of failing: the gateway would then validate or send
// on a value the client never wrote. Such documents are refused instead.
package jsontext

import "unicode/utf8"

// Valid reports whether b is UTF-8 and every \u escape inside its strings
// is a character or a complete surrogate pair.
func Valid(b []byte) bool {
	if !utf8.Valid(b) {
		return false
	}
	inString := false
	for i := 0; i < len(b); i++ {
		c := b[i]
		if !inString {
			if c == '"' {
				inString = true
			}
			continue
		}
		switch c {
		case '"':
			inString = false
		case '\\':
			if i+1 >= len(b) {
				return false
			}
			if b[i+1] != 'u' {
				i++ // \" \\ \n …: the decoder checks them
				continue
			}
			r, ok := hex4(b, i+2)
			if !ok {
				return true // malformed: the decoder refuses it
			}
			switch {
			case r >= 0xDC00 && r <= 0xDFFF:
				return false // low surrogate alone
			case r >= 0xD800 && r <= 0xDBFF:
				lo, ok := hex4(b, i+8)
				if i+7 >= len(b) || b[i+6] != '\\' || b[i+7] != 'u' || !ok || lo < 0xDC00 || lo > 0xDFFF {
					return false // high surrogate without its pair
				}
				i += 11
			default:
				i += 5
			}
		}
	}
	return true
}

func hex4(b []byte, at int) (rune, bool) {
	if at+4 > len(b) {
		return 0, false
	}
	var r rune
	for _, c := range b[at : at+4] {
		switch {
		case c >= '0' && c <= '9':
			r = r<<4 | rune(c-'0')
		case c >= 'a' && c <= 'f':
			r = r<<4 | rune(c-'a'+10)
		case c >= 'A' && c <= 'F':
			r = r<<4 | rune(c-'A'+10)
		default:
			return 0, false
		}
	}
	return r, true
}
