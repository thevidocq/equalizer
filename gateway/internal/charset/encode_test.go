package charset

import (
	"errors"
	"testing"
)

func TestFromUTF8(t *testing.T) {
	cases := []struct {
		cs, in, want string
		err          error
	}{
		{UTF8, "café €", "café €", nil},
		{ISO88591, "café è", "caf\xe9 \xe8", nil},
		{ISO88591, "price €5", "", ErrUnrepresentable},
		{Windows1252, "€5 “ok”", "\x805 \x93ok\x94", nil},
		{Windows1252, "\u0085", "", ErrUnrepresentable},
		{ISO885915, "€ œ", "\xa4 \xbd", nil},
		{ISO885915, "¤", "", ErrUnrepresentable},
		{ISO88591, "日本", "", ErrUnrepresentable},
		{"koi8-r", "x", "", ErrUnsupported},
	}
	for _, c := range cases {
		got, err := FromUTF8(c.cs, c.in)
		if !errors.Is(err, c.err) || got != c.want {
			t.Errorf("FromUTF8(%s, %q) = %q, %v; want %q, %v", c.cs, c.in, got, err, c.want, c.err)
		}
	}
	// Round trip through every supported encoding.
	for _, cs := range []string{ISO88591, ISO885915, Windows1252} {
		enc, err := FromUTF8(cs, "àèìòù ÀÉ ç ñ")
		if err != nil {
			t.Fatal(err)
		}
		dec, _ := ToUTF8(cs, []byte(enc))
		if string(dec) != "àèìòù ÀÉ ç ñ" {
			t.Errorf("%s round trip: %q", cs, dec)
		}
	}
}
