package charset

import (
	"errors"
	"fmt"
	"sync"
	"unicode/utf8"
)

// ErrUnrepresentable reports a character the target encoding cannot hold.
var ErrUnrepresentable = errors.New("character not representable in the target encoding")

// FromUTF8 converts a UTF-8 string to the given encoding. Pages are served in
// UTF-8, so browsers submit forms in UTF-8: values must go back to the legacy
// application in its own encoding. A character the encoding cannot hold is
// an error, never a silent substitution.
func FromUTF8(name, s string) (string, error) {
	if !utf8.ValidString(s) {
		return "", fmt.Errorf("%w: invalid UTF-8", ErrUnrepresentable)
	}
	var reverse map[rune]byte
	switch name {
	case UTF8:
		return s, nil
	case ISO88591:
	case ISO885915:
		reverse = reverseISO885915
	case Windows1252:
		reverse = reverseWindows1252
	default:
		return fromTable(name, s)
	}
	out := make([]byte, 0, len(s))
	for _, r := range s {
		switch {
		case r < 0x80:
			out = append(out, byte(r))
		case reverse[r] != 0:
			out = append(out, reverse[r])
		case r <= 0xFF && !overridden(name, byte(r)):
			out = append(out, byte(r))
		default:
			return "", ErrUnrepresentable
		}
	}
	return string(out), nil
}

// overridden reports whether a Latin-1 position holds a different character
// in the encoding (so the Latin-1 code point is not available there).
func overridden(name string, b byte) bool {
	switch name {
	case ISO885915:
		_, ok := iso885915[b]
		return ok
	case Windows1252:
		// Windows-1252 has no C1 control characters.
		return b >= 0x80 && b <= 0x9F
	}
	return false
}

var (
	reverseWindows1252 = invert(windows1252)
	reverseISO885915   = invert(iso885915)
)

var (
	reverseOnce   sync.Once
	reverseTables map[string]map[rune]byte
)

// fromTable encodes with one of the generated single-byte tables.
func fromTable(name, s string) (string, error) {
	if _, ok := tables[name]; !ok {
		return "", fmt.Errorf("%w: %q", ErrUnsupported, name)
	}
	reverseOnce.Do(func() {
		reverseTables = make(map[string]map[rune]byte, len(tables))
		for n, tb := range tables {
			m := make(map[rune]byte, 128)
			for i, r := range tb {
				if r != 0xFFFD {
					m[r] = byte(0x80 + i)
				}
			}
			reverseTables[n] = m
		}
	})
	rev := reverseTables[name]
	out := make([]byte, 0, len(s))
	for _, r := range s {
		switch b, ok := rev[r]; {
		case r < 0x80:
			out = append(out, byte(r))
		case ok:
			out = append(out, b)
		default:
			return "", ErrUnrepresentable
		}
	}
	return string(out), nil
}

func invert(m map[byte]rune) map[rune]byte {
	out := make(map[rune]byte, len(m))
	for b, r := range m {
		out[r] = b
	}
	return out
}
