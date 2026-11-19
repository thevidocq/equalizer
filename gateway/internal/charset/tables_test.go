package charset

import "testing"

func TestTables(t *testing.T) {
	cases := map[string]struct {
		raw  []byte
		text string
	}{
		"windows-1250": {[]byte{0x8A, 0xE8, 0x9E}, "Ščž"}, // Czech, Slovak, Polish sites
		"windows-1251": {[]byte{0xCF, 0xF0, 0xE8}, "При"}, // Cyrillic
		"iso-8859-7":   {[]byte{0xC1, 0xE8, 0xDE}, "Αθή"}, // Greek
		"koi8-r":       {[]byte{0xF0, 0xD2, 0xC9}, "При"},
	}
	for name, c := range cases {
		got, err := ToUTF8(name, c.raw)
		if err != nil || string(got) != c.text {
			t.Errorf("%s decode: %q %v", name, got, err)
		}
		back, err := FromUTF8(name, c.text)
		if err != nil || back != string(c.raw) {
			t.Errorf("%s encode: %x %v", name, back, err)
		}
	}
	if cs, err := Detect("text/html; charset=CP1251", nil); err != nil || cs != "windows-1251" {
		t.Errorf("alias: %q %v", cs, err)
	}
	if _, err := FromUTF8("windows-1251", "€ ok, 中 no"); err == nil {
		t.Error("a character outside the encoding was accepted")
	}
}
