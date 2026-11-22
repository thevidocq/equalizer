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
