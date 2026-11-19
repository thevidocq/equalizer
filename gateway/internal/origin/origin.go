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
			r.keep = max(r.keep, len(f[0])-1)
		}
	}
	if len(r.from) == 0 {
		return nil
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
	for i, f := range r.from {
		if bytes.Contains(b, f) {
			b = bytes.ReplaceAll(b, f, r.to[i])
		}
	}
	return b
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

func (s *writer) Write(p []byte) (int, error) {
	s.buf = s.r.Bytes(append(s.buf, p...))
	if cut := len(s.buf) - s.r.keep; cut > 0 {
		if _, err := s.w.Write(s.buf[:cut]); err != nil {
			return 0, err
		}
		s.buf = append(s.buf[:0], s.buf[cut:]...)
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
