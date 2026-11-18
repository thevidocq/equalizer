package charset

import (
	"errors"
	"testing"
)

func TestDetect(t *testing.T) {
	cases := []struct {
		ct, body, want string
	}{
		{"text/html; charset=ISO-8859-1", "", ISO88591},
		{"text/html", `<html><head><meta charset="windows-1252">`, Windows1252},
		{"text/html", `<meta http-equiv="Content-Type" content="text/html; charset=iso-8859-15">`, ISO885915},
		{"text/html", "città", UTF8},
		{"text/html", "citt\xe0", Windows1252},
	}
	for _, c := range cases {
		got, err := Detect(c.ct, []byte(c.body))
		if err != nil || got != c.want {
			t.Errorf("Detect(%q, %q) = %q, %v; atteso %q", c.ct, c.body, got, err, c.want)
		}
	}
	if _, err := Detect("text/html; charset=shift_jis", nil); !errors.Is(err, ErrUnsupported) {
		t.Errorf("codifica non supportata accettata: %v", err)
	}
}

func TestToUTF8(t *testing.T) {
	cases := []struct {
		cs, in, want string
	}{
		{ISO88591, "citt\xe0 \xe8", "città è"},
		{Windows1252, "\x93ciao\x94 \x80", "“ciao” €"},
		{ISO885915, "\xa4", "€"},
		{UTF8, "già", "già"},
	}
	for _, c := range cases {
		got, err := ToUTF8(c.cs, []byte(c.in))
		if err != nil || string(got) != c.want {
			t.Errorf("ToUTF8(%s, %q) = %q, %v; atteso %q", c.cs, c.in, got, err, c.want)
		}
	}
}
