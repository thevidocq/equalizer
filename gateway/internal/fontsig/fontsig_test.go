package fontsig

import "testing"

var (
	woff  = []byte("wOFF\x00\x01\x00\x00rest")
	woff2 = []byte("wOF2\x00\x01\x00\x00rest")
	ttf   = []byte{0, 1, 0, 0, 0, 12}
	otf   = []byte("OTTO\x00\x0c")
)

func TestSniff(t *testing.T) {
	for want, body := range map[string][]byte{WOFF: woff, WOFF2: woff2, TTF: ttf, OTF: otf} {
		if got := Sniff(body); got != want {
			t.Errorf("Sniff = %q, want %q", got, want)
		}
	}
	if got := Sniff([]byte("<html>")); got != "" {
		t.Errorf("Sniff(html) = %q", got)
	}
}

func TestResolve(t *testing.T) {
	allowed := []string{"text/css", "font/woff", "font/woff2", "application/font-woff"}
	cases := []struct {
		name, declared string
		body           []byte
		want           string
	}{
		{"old woff name", "application/x-font-woff", woff, WOFF},
		{"octet-stream woff", "application/octet-stream", woff, WOFF},
		{"octet-stream woff2", "application/octet-stream", woff2, WOFF2},
		{"ttf not in the map", "application/octet-stream", ttf, ""},
		{"not a font", "application/octet-stream", []byte("<script>"), ""},
		{"declared woff2, body woff", "font/woff2", woff, ""},
		{"declared html, body woff", "text/html", woff, ""},
		{"declared javascript, body woff", "application/javascript", woff, ""},
	}
	for _, c := range cases {
		if got := Resolve(c.declared, c.body, allowed); got != c.want {
			t.Errorf("%s: Resolve = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestSfntFamily(t *testing.T) {
	allowed := []string{"font/ttf"}
	for _, c := range []struct{ declared, body, want string }{
		{"font/ttf", "OTTO\x00\x0c", OTF},
		{"application/font-sfnt", "ttcf\x00\x02", TTC},
		{"application/octet-stream", "ttcf\x00\x02", TTC},
		{"font/ttf", "wOFF\x00\x01", ""}, // a WOFF is not an sfnt font: the map allows only TTF
	} {
		if got := Resolve(c.declared, []byte(c.body), allowed); got != c.want {
			t.Errorf("Resolve(%q, %q) = %q, want %q", c.declared, c.body, got, c.want)
		}
	}
}

func TestEOTNeedsItsVersion(t *testing.T) {
	forged := append(make([]byte, 34), []byte("LP<html>")...)
	if Sniff(forged) != "" {
		t.Error("34 bytes and LP are no EOT")
	}
	real := make([]byte, 40)
	real[8], real[9], real[10], real[11] = 0x01, 0x00, 0x02, 0x00 // 0x00020001
	real[34], real[35] = 0x4C, 0x50
	if Sniff(real) != EOT {
		t.Error("a real EOT header")
	}
}
