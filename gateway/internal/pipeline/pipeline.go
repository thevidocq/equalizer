// Package pipeline applies faithful rendering to legacy HTML pages: it
// converts them to UTF-8, parses the HTML5, applies a series of independent
// steps and serialises the page again. On screen the page stays identical.
package pipeline

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"

	"github.com/thevidocq/equalizer/gateway/internal/charset"
	html "github.com/thevidocq/equalizer/gateway/internal/thirdparty/xhtml"
	"github.com/thevidocq/equalizer/gateway/internal/thirdparty/xhtml/atom"
)

// URLMapper translates legacy URLs into public URLs (legacyurl.Rewriter).
type URLMapper interface {
	Internal(u *url.URL) bool
	Public(u *url.URL) (string, bool)
	Fallback(u *url.URL) string
}

// Context is what a step knows about the page being processed.
type Context struct {
	PageURL *url.URL // legacy URL the page comes from
	URLs    URLMapper
	Stats   Stats
}

// Stats summarises what the pipeline did, for logs and request tracing.
type Stats struct {
	Charset       string
	Rewritten     int
	Unmapped      int
	Removed       int
	UnmappedPaths []string // at most 20, without query
}

// NoteUnmapped records an internal URL without a public route.
func (s *Stats) NoteUnmapped(u *url.URL) {
	s.Unmapped++
	if len(s.UnmappedPaths) < 20 {
		s.UnmappedPaths = append(s.UnmappedPaths, u.Path)
	}
}

// Step is a faithful-rendering step. Each step modifies the tree and knows
// nothing about the others.
type Step interface {
	Name() string
	Apply(doc *html.Node, ctx *Context) error
}

// Pipeline is a configured sequence of steps.
type Pipeline struct {
	steps []Step
}

// New creates a pipeline with the steps in the given order.
func New(steps ...Step) *Pipeline {
	return &Pipeline{steps: steps}
}

// ContentType is the type declared to the client for processed pages.
const ContentType = "text/html; charset=utf-8"

// Render processes an HTML page and returns it in UTF-8.
func (p *Pipeline) Render(body []byte, contentType string, ctx *Context) ([]byte, error) {
	cs, err := charset.Detect(contentType, body)
	if err != nil {
		return nil, err
	}
	ctx.Stats.Charset = cs
	utf8Body, err := charset.ToUTF8(cs, body)
	if err != nil {
		return nil, err
	}
	doc, err := html.Parse(bytes.NewReader(utf8Body))
	if err != nil {
		return nil, fmt.Errorf("parsing HTML: %w", err)
	}
	declareUTF8(doc)
	for _, s := range p.steps {
		if err := s.Apply(doc, ctx); err != nil {
			return nil, fmt.Errorf("step %s: %w", s.Name(), err)
		}
	}
	var out bytes.Buffer
	if err := html.Render(&out, doc); err != nil {
		return nil, fmt.Errorf("serialisation: %w", err)
	}
	return out.Bytes(), nil
}

// IsHTML reports whether a Content-Type must go through the pipeline.
func IsHTML(contentType string) bool {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	return strings.HasPrefix(ct, "text/html") || strings.HasPrefix(ct, "application/xhtml+xml")
}

// Walk visits all nodes depth first. fn returns false to skip the children.
func Walk(n *html.Node, fn func(*html.Node) bool) {
	if !fn(n) {
		return
	}
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling // fn may remove c
		Walk(c, fn)
		c = next
	}
}

// declareUTF8 aligns the encoding declarations with the converted page.
func declareUTF8(doc *html.Node) {
	Walk(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode || n.DataAtom != atom.Meta {
			return true
		}
		for i, a := range n.Attr {
			switch strings.ToLower(a.Key) {
			case "charset":
				n.Attr[i].Val = "utf-8"
			case "content":
				if isContentTypeMeta(n) {
					n.Attr[i].Val = ContentType
				}
			}
		}
		return false
	})
}

func isContentTypeMeta(n *html.Node) bool {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, "http-equiv") && strings.EqualFold(strings.TrimSpace(a.Val), "content-type") {
			return true
		}
	}
	return false
}
