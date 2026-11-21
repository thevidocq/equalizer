package rebuild

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"reflect"
	"strings"
	"testing"
)

// Rebuilding never changes what the legacy application reads: the pairs
// read back from the rebuilt query are the pairs read from the client's.
func FuzzPairs(f *testing.F) {
	for _, s := range []string{"a=1&b=2&a=3", "a[]=1&a[]=2", "q=%27+or+1%3D1", "x=%2527", "a=%u0027", "k=v;w=z", "=", "a=%ff", "%00=x", "a+b=c+d&&&e"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		pairs, err := ParsePairs(raw)
		if err != nil {
			return
		}
		out, err := EncodePairs(pairs, "")
		if err != nil {
			return
		}
		again, err := ParsePairs(out)
		if err != nil {
			t.Fatalf("rebuilt query unreadable: %q -> %q: %v", raw, out, err)
		}
		if len(pairs) == 0 && len(again) == 0 {
			return
		}
		if !reflect.DeepEqual(pairs, again) {
			t.Fatalf("values changed: %q -> %q\n%v\n%v", raw, out, pairs, again)
		}
	})
}

// The rebuilt JSON reads as the same tokens, in the same order, and
// rebuilding it again changes nothing.
func FuzzJSON(f *testing.F) {
	for _, s := range []string{`{"a":1,"b":[true,null,"x"]}`, `{"a":1,"a":2}`, `"\ud800"`, `1e999`, `{"<":"&"}`, `[[[[]]]]`, `{"a":"\u0000"}`, `12345678901234567890123`} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, body []byte) {
		out, err := JSON(body)
		if err != nil {
			return
		}
		if !json.Valid(out) {
			t.Fatalf("invalid output %q -> %q", body, out)
		}
		if a, b := tokens(t, body), tokens(t, out); !reflect.DeepEqual(a, b) {
			t.Fatalf("tokens changed: %q -> %q", body, out)
		}
		again, err := JSON(out)
		if err != nil || !bytes.Equal(again, out) {
			t.Fatalf("not stable: %q -> %q -> %q (%v)", body, out, again, err)
		}
	})
}

func tokens(t *testing.T, b []byte) []any {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var out []any
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return out
		}
		if err != nil {
			t.Fatalf("unreadable %q: %v", b, err)
		}
		out = append(out, tok)
	}
}

// Rebuilt XML is accepted again as it is: no DOCTYPE, no instruction
// other than the declaration, balanced.
func FuzzXML(f *testing.F) {
	for _, s := range []string{`<a b="1">x</a>`, `<?xml version="1.0"?><a/>`, `<!DOCTYPE a [<!ENTITY x SYSTEM "file:///etc/passwd">]><a>&x;</a>`, `<a><?php system('id');?></a>`, `<p:a xmlns:p="u">t</p:a>`, `<a>&amp;&lt;</a>`} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, body []byte) {
		out, err := XML(body)
		if err != nil {
			return
		}
		if bytes.Contains(out, []byte("<!")) {
			t.Fatalf("a directive survived: %q -> %q", body, out)
		}
		if i := bytes.Index(out, []byte("<?")); i >= 0 && !bytes.HasPrefix(out[i:], []byte(`<?xml version="1.0" encoding="UTF-8"?>`)) {
			t.Fatalf("an instruction survived: %q -> %q", body, out)
		}
		again, err := XML(out)
		if err != nil || !bytes.Equal(again, out) {
			t.Fatalf("not stable: %q -> %q -> %q (%v)", body, out, again, err)
		}
	})
}

// A rebuilt multipart form reads back as the same field names and values;
// files keep their bytes; a runnable file never comes out.
func FuzzMultipart(f *testing.F) {
	const boundary = "xYz"
	seed := func(parts ...string) []byte {
		return []byte("--" + boundary + "\r\n" + strings.Join(parts, "\r\n--"+boundary+"\r\n") + "\r\n--" + boundary + "--\r\n")
	}
	f.Add(seed("Content-Disposition: form-data; name=\"a\"\r\n\r\n1"))
	f.Add(seed("Content-Disposition: form-data; name=\"a\\\"b\"\r\n\r\nx", "Content-Disposition: form-data; name=\"f\"; filename=\"c.txt\"\r\nContent-Type: text/plain\r\n\r\nhello"))
	f.Add(seed("Content-Disposition: form-data; name=\"f\"; filename=\"x.php\"\r\n\r\n<?php echo 1;"))
	f.Add(seed("Content-Disposition: form-data; name=\"f\"; filename*=UTF-8''x.php\r\n\r\nx"))
	f.Fuzz(func(t *testing.T, body []byte) {
		var out bytes.Buffer
		ctype, err := Multipart(&out, bytes.NewReader(body), boundary, "")
		if err != nil {
			return
		}
		in, ok := readParts(bytes.NewReader(body), boundary)
		if !ok {
			return
		}
		_, params, err := mime.ParseMediaType(ctype)
		if err != nil {
			t.Fatalf("content type %q: %v", ctype, err)
		}
		got, ok := readParts(bytes.NewReader(out.Bytes()), params["boundary"])
		if !ok {
			t.Fatalf("rebuilt form unreadable: %q", out.Bytes())
		}
		if len(in) != len(got) {
			t.Fatalf("parts changed: %d -> %d", len(in), len(got))
		}
		for i := range in {
			if in[i].value != got[i].value {
				t.Fatalf("part %d value changed", i)
			}
			if strings.ContainsAny(in[i].name, "\"\r\n") {
				continue // written as browsers do (%22, %0D, %0A)
			}
			if in[i].name != got[i].name {
				t.Fatalf("part %d name %q -> %q", i, in[i].name, got[i].name)
			}
			if runnableName(got[i].file) {
				t.Fatalf("runnable file passed: %q", got[i].file)
			}
		}
	})
}

type part struct{ name, file, value string }

func readParts(r io.Reader, boundary string) ([]part, bool) {
	mr := multipart.NewReader(r, boundary)
	var out []part
	for {
		p, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			return out, true
		}
		if err != nil {
			return nil, false
		}
		b, err := io.ReadAll(p)
		if err != nil {
			return nil, false
		}
		out = append(out, part{p.FormName(), p.FileName(), string(b)})
	}
}
