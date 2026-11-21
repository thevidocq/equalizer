package jsontext

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestValid(t *testing.T) {
	for in, want := range map[string]bool{
		`{"a":"b"}`:        true,
		`{"a":"è"}`:        true,
		`{"a":"😀"}`:        true,
		`{"a":"\\ud800"}`:  true, // an escaped backslash, then text
		`{"a":"\ud800"}`:   false,
		`{"a":"\udc00"}`:   false,
		`{"a":"\ud800x"}`:  false,
		`{"a":"\ud800A"}`:  false,
		"{\"a\":\"\xff\"}": false,
		`{"\ud800":1}`:     false,
		`[1,2,"ok"]`:       true,
	} {
		if got := Valid([]byte(in)); got != want {
			t.Errorf("Valid(%s) = %v, want %v", in, got, want)
		}
	}
}

// A document Valid accepts never decodes to U+FFFD unless it wrote one.
func FuzzValid(f *testing.F) {
	for _, s := range []string{`{"a":"\ud800"}`, `"😀"`, `["\\u"]`, `{"k":"�"}`} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		if !Valid(b) {
			return
		}
		var v any
		if json.Unmarshal(b, &v) != nil {
			return
		}
		out, _ := json.Marshal(v)
		if strings.Contains(string(out), "�") && !strings.Contains(string(b), "�") &&
			!strings.Contains(strings.ToLower(string(b)), `�`) {
			t.Fatalf("accepted a document that decodes to U+FFFD: %q", b)
		}
		if !utf8.Valid(out) {
			t.Fatal("not UTF-8")
		}
	})
}
