package rebuild

import (
	"errors"
	"testing"
)

// The rebuilt query carries exactly what the client wrote, decoded once:
// never decoded twice, never "corrected", never mapped to a look-alike.
func TestQueryNeverTransformsValues(t *testing.T) {
	for _, c := range []struct{ raw, cs, want, refused string }{
		{raw: "x=%2527", want: "x=%2527"},                              // stays %27 for the legacy app: one decoding only
		{raw: "a=1&a=2", want: "a=1&a=2"},                              // repetitions kept, in order (PHP arrays)
		{raw: "a=1;b=2", want: "a=1%3Bb%3D2"},                          // ";" is not a separator: one value
		{raw: "q=%EF%BC%87", want: "q=%EF%BC%87"},                      // a full-width apostrophe stays one
		{raw: "q=%EF%BC%87", cs: "iso-8859-1", refused: ReasonCharset}, // never best-fit to "'"
		{raw: "q=%CA%BC", cs: "windows-1252", refused: ReasonCharset},  // modifier apostrophe: same
		{raw: "a=%00", refused: ReasonNUL},
		{raw: "a=%ff", refused: ReasonCharset},
		{raw: "a=%zz", refused: ReasonMalformed},
		{raw: "a=%u0027", refused: ReasonMalformed}, // IIS-style escapes are not decoded
	} {
		got, err := Query(c.raw, c.cs)
		var re *Error
		switch {
		case c.refused != "":
			if !errors.As(err, &re) || re.Reason != c.refused {
				t.Errorf("%q (%s): %q %v, want refusal %s", c.raw, c.cs, got, err, c.refused)
			}
		case err != nil || got != c.want:
			t.Errorf("%q: %q %v, want %q", c.raw, got, err, c.want)
		}
	}
}

func TestJSONRefusesWhatParsersReadDifferently(t *testing.T) {
	for in, ok := range map[string]bool{
		`{"role":"user","role":"admin"}`:       false, // first for some parsers, last for others
		`{"a":{"b":1,"b":2}}`:                  false,
		`{"a":"\ud800"}`:                       false, // would become U+FFFD
		"{\"a\":\"\xc0\xa7\"}":                 false, // overlong UTF-8 for "'"
		`{"n":123456789012345678901234567890}`: true,
		`{"a":"'"}`:                            true,
	} {
		out, err := JSON([]byte(in))
		if (err == nil) != ok {
			t.Errorf("%s: %s %v", in, out, err)
		}
	}
	out, _ := JSON([]byte(`{"n":123456789012345678901234567890,"f":1.50}`))
	if string(out) != `{"n":123456789012345678901234567890,"f":1.50}` {
		t.Errorf("numbers changed: %s", out)
	}
}

func TestXMLDropsInstructions(t *testing.T) {
	out, err := XML([]byte(`<?xml version="1.0"?><a><?php system('id'); ?>x</a>`))
	if err != nil || string(out) != `<?xml version="1.0" encoding="UTF-8"?><a>x</a>` {
		t.Errorf("%s %v", out, err)
	}
}
