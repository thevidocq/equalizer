package origin

import (
	"bytes"
	"net/url"
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
		src := `{"a":"http://10.0.0.5:8080/x","b":"https:\/\/10.0.0.5:8080\/y","c":"http://10.0.0.5:80801/","d":"http://10.0.0.5:8080"}`
		for i := 0; i < len(src); i += cut {
			_, _ = w.Write([]byte(src[i:min(i+cut, len(src))]))
		}
		_ = w.Close()
		want := `{"a":"https://portal.acme.test/x","b":"https:\/\/portal.acme.test\/y","c":"http://10.0.0.5:80801/","d":"https://portal.acme.test"}`
		if out.String() != want {
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

func TestReverse(t *testing.T) {
	r := NewReverse(mustURL("http://www.acme.test"), "www.acme.test")
	in := `siteurl=https%3A%2F%2Fwww.acme.test&home=https%3a%2f%2fwww.acme.test%2F {"url":"https:\/\/www.acme.test"} https://www.acme.test/x https://www.acme.testing/ https://www.acme.test.evil.example/ https://www.acme.test.`
	want := `siteurl=http%3A%2F%2Fwww.acme.test&home=http%3a%2f%2fwww.acme.test%2F {"url":"http:\/\/www.acme.test"} http://www.acme.test/x https://www.acme.testing/ https://www.acme.test.evil.example/ http://www.acme.test.`
	if got := string(r.Bytes([]byte(in))); got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
	if NewReverse(mustURL("https://www.acme.test"), "www.acme.test") != nil {
		t.Error("same origin: nothing to undo")
	}
}

func mustURL(s string) *url.URL { u, _ := url.Parse(s); return u }
