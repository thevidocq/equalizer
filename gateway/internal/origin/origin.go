// Package origin rewrites the legacy application's own address into the
// public one inside text answers: inline scripts and styles, JSON (REST
// APIs), JavaScript, CSS and feeds. A legacy site that only knows itself as
// http://… or by an internal address would otherwise make the browser load
// http resources from an https page (blocked as mixed content) or reveal
// the internal address.
package origin

import (
	"bytes"
	"io"
	"mime"
	"net/url"
	"strings"
)

// Rewriter replaces the legacy origins with the public one.
type Rewriter struct {
	from [][]byte
	to   [][]byte
	keep int // bytes kept between writes so a split origin is still found
}

// New builds the replacements: the legacy base's scheme and host, its
// other scheme, both also with JSON's escaped slashes. Nil when there is
// nothing to replace (legacy and public origins are the same).
func New(legacy *url.URL, publicHost string) *Rewriter {
	if legacy == nil || publicHost == "" {
		return nil
	}
	public := "https://" + publicHost
	r := &Rewriter{}
	for _, scheme := range []string{"http", "https"} {
		o := scheme + "://" + legacy.Host
		if strings.EqualFold(o, public) {
			continue
		}
		for _, f := range [][2]string{{o, public}, {strings.ReplaceAll(o, "/", `\/`), strings.ReplaceAll(public, "/", `\/`)}} {
			r.from = append(r.from, []byte(f[0]))
			r.to = append(r.to, []byte(f[1]))
			r.keep = max(r.keep, len(f[0])+1) // + the bytes that decide where the name ends
		}
	}
	if len(r.from) == 0 {
		return nil
	}
	return r
}

// NewReverse undoes New on what the browser sends: the public origin in a
// query or a body (plain, JSON-escaped or percent-encoded) becomes the
// legacy one again, so a form showing the public address (WordPress's site
// address, a profile's website) saves the legacy application's own value.
func NewReverse(legacy *url.URL, publicHost string) *Rewriter {
	if legacy == nil || publicHost == "" {
		return nil
	}
	public := "https://" + publicHost
	target := legacy.Scheme + "://" + legacy.Host
	if strings.EqualFold(public, target) {
		return nil
	}
	r := &Rewriter{}
	enc := func(s string) string { return strings.NewReplacer(":", "%3A", "/", "%2F").Replace(s) }
	for _, f := range [][2]string{
		{public, target},
		{strings.ReplaceAll(public, "/", `\/`), strings.ReplaceAll(target, "/", `\/`)},
		{enc(public), enc(target)},
		{strings.ToLower(enc(public)), strings.ToLower(enc(target))},
	} {
		r.from = append(r.from, []byte(f[0]))
		r.to = append(r.to, []byte(f[1]))
		r.keep = max(r.keep, len(f[0])+1) // + the bytes that decide where the name ends
	}
	return r
}

// Text reports whether a content type is rewritten.
func Text(contentType string) bool {
	mt, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	switch {
	case mt == "text/html", mt == "text/css", mt == "text/javascript", mt == "application/javascript",
		mt == "application/x-javascript", mt == "application/json", mt == "text/xml", mt == "application/xml",
		strings.HasSuffix(mt, "+json"), strings.HasSuffix(mt, "+xml"):
		return true
	}
	return false
}

// Bytes rewrites a whole body.
func (r *Rewriter) Bytes(b []byte) []byte {
	if r == nil {
		return b
	}
	out, _ := r.scan(b, len(b))
	return out
}

// scan rewrites the origins that start before limit and returns the result
// with the number of bytes of b it covers (at least limit). Every pattern
// starts with "h", and a match is replaced only when the host name ends
// there: www.acme.test, not www.acme.testing or www.acme.test.other.example.
func (r *Rewriter) scan(b []byte, limit int) ([]byte, int) {
	var out []byte
	last, i := 0, 0
	for i < limit {
		j := bytes.IndexByte(b[i:limit], 'h')
		if j < 0 {
			break
		}
		at := i + j
		i = at + 1
		for k, f := range r.from {
			end := at + len(f)
			if !bytes.HasPrefix(b[at:], f) || end < len(b) && hostByte(b, end) {
				continue
			}
			out = append(append(out, b[last:at]...), r.to[k]...)
			last, i = end, end
			break
		}
	}
	covered := max(limit, last)
	if out == nil {
		return b[:covered], covered
	}
	return append(out, b[last:covered]...), covered
}

// hostByte reports whether b[i] continues a host name: a letter, digit or
// hyphen, or a dot followed by one of those.
func hostByte(b []byte, i int) bool {
	isName := func(c byte) bool {
		return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-'
	}
	if b[i] == '.' {
		return i+1 < len(b) && isName(b[i+1])
	}
	return isName(b[i])
}

// Writer rewrites a stream; Close writes what it still holds.
func (r *Rewriter) Writer(w io.Writer) io.WriteCloser {
	return &writer{r: r, w: w}
}

type writer struct {
	r   *Rewriter
	w   io.Writer
	buf []byte
}

// Write keeps the last bytes: an origin starting there may not be whole
// yet, or its host name may go on in the next write.
func (s *writer) Write(p []byte) (int, error) {
	s.buf = append(s.buf, p...)
	if limit := len(s.buf) - s.r.keep; limit > 0 {
		out, n := s.r.scan(s.buf, limit)
		if _, err := s.w.Write(out); err != nil {
			return 0, err
		}
		s.buf = append(s.buf[:0], s.buf[n:]...)
	}
	return len(p), nil
}

func (s *writer) Close() error {
	if len(s.buf) == 0 {
		return nil
	}
	_, err := s.w.Write(s.r.Bytes(s.buf))
	s.buf = nil
	return err
}
