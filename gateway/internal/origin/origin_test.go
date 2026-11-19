package origin

import (
	"bytes"
	"net/url"
	"strings"
	"testing"
)

func TestRewrite(t *testing.T) {
	base, _ := url.Parse("http://www.acme.test")
	r := New(base, "www.acme.test")
	in := `<link href='http://www.acme.test/a.css'><script>var s={"root":"http:\/\/www.acme.test\/wp-json\/"}</script>https://www.acme.test/x http://other.example/`
	want := `<link href='https://www.acme.test/a.css'><script>var s={"root":"https:\/\/www.acme.test\/wp-json\/"}</script>https://www.acme.test/x http://other.example/`
	if got := string(r.Bytes([]byte(in))); got != want {
		t.Fatalf("got %s", got)
	}
	// An internal address, in a stream cut anywhere.
	internal, _ := url.Parse("http://10.0.0.5:8080")
	r = New(internal, "portal.acme.test")
	for cut := 1; cut < 40; cut++ {
		var out bytes.Buffer
		w := r.Writer(&out)
		src := `{"a":"http://10.0.0.5:8080/x","b":"https:\/\/10.0.0.5:8080\/y"}`
		for i := 0; i < len(src); i += cut {
			_, _ = w.Write([]byte(src[i:min(i+cut, len(src))]))
		}
		_ = w.Close()
		if strings.Contains(out.String(), "10.0.0.5") || !strings.Contains(out.String(), `https:\/\/portal.acme.test\/y`) {
			t.Fatalf("cut %d: %s", cut, out.String())
		}
	}
	if New(mustURL("https://www.acme.test"), "www.acme.test") == nil {
		t.Error("the http form of the same name must still be rewritten")
	}
	if !Text("application/json; charset=utf-8") || Text("image/png") || !Text("application/rss+xml") {
		t.Error("Text")
	}
}

func mustURL(s string) *url.URL { u, _ := url.Parse(s); return u }
