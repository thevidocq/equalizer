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

// FormBinder recognises legacy forms that post to a declared form and seals
// their server-side state into a token (see the formmap and formtoken
// packages). The server provides one per request.
type FormBinder interface {
	Match(action *url.URL, hidden map[string]string) (FormMatch, bool)
	Token(m FormMatch, charset string) (string, error)
}

// FormMatch is a legacy form recognised as a declared form.
type FormMatch struct {
	FormID string
	Action string            // public action URL
	Params map[string]string // path parameters of the public action
	Hidden map[string]string // hidden values kept server-side
	Rename map[string]string // legacy input name → public field name
}

// Context is what a step knows about the page being processed.
type Context struct {
	PageURL *url.URL   // legacy URL the page comes from
	URLs    URLMapper  // legacy → public URLs
	Forms   FormBinder // nil: forms are not bound and only their URLs are rewritten
	Stats   Stats

	final map[*html.Node]bool // elements whose URLs are already public
}

// MarkFinal records that an element's URL attributes are already public:
// later steps must not rewrite them again.
func (c *Context) MarkFinal(n *html.Node) {
	if c.final == nil {
		c.final = map[*html.Node]bool{}
	}
	c.final[n] = true
}

// Final reports whether an element was marked with MarkFinal.
func (c *Context) Final(n *html.Node) bool { return c.final[n] }

// BaseURL returns the URL relative links resolve against: the page URL,
// changed by the first <base href> if present.
func BaseURL(doc *html.Node, pageURL *url.URL) *url.URL {
	base := pageURL
	found := false
	Walk(doc, func(n *html.Node) bool {
		if found {
			return false
		}
		if n.Type == html.ElementNode && n.DataAtom == atom.Base {
			if href, ok := Attr(n, "href"); ok {
				if u, err := url.Parse(strings.TrimSpace(href)); err == nil {
					base = pageURL.ResolveReference(u)
				}
				found = true
			}
		}
		return true
	})
	return base
}

// Attr returns the value of an attribute (case-insensitive name).
func Attr(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val, true
		}
	}
	return "", false
}

// SetAttr sets an attribute, adding it if missing.
func SetAttr(n *html.Node, key, val string) {
	for i, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			n.Attr[i].Val = val
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: val})
}

// DelAttr removes an attribute.
func DelAttr(n *html.Node, key string) {
	out := n.Attr[:0]
	for _, a := range n.Attr {
		if !strings.EqualFold(a.Key, key) {
			out = append(out, a)
		}
	}
	n.Attr = out
}

// Stats summarises what the pipeline did, for logs and request tracing.
type Stats struct {
	Charset       string
	Rewritten     int
	Unmapped      int
	Removed       int
	Forms         int      // forms bound to a declared form
	FormsUnmapped int      // POST forms that match no declared form
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
	if !isDocument(utf8Body) {
		out, err := p.renderFragment(utf8Body, ctx)
		if err != nil {
			return nil, fmt.Errorf("fragment: %w", err)
		}
		return out, nil
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
