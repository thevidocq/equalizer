package pipeline

import (
	"bytes"
	"io"

	html "github.com/thevidocq/equalizer/gateway/internal/thirdparty/xhtml"
	"github.com/thevidocq/equalizer/gateway/internal/thirdparty/xhtml/atom"
)

// Answers declared as HTML are not always pages: AJAX endpoints send
// fragments (table rows, a list, a form) and some send JSON or plain text
// as text/html. A page is rendered as a page; a fragment is rendered as a
// fragment, without the html, head and body a parser adds; what is not
// markup at all is not rendered (Markup).

var bom = []byte("\xef\xbb\xbf")

func start(body []byte) []byte {
	return bytes.TrimLeft(bytes.TrimPrefix(body, bom), " \t\r\n\f")
}

// Markup reports whether an answer declared as HTML is markup: it starts
// with a tag. JSON or text sent as text/html is not, and must be passed on
// as it is.
func Markup(body []byte) bool {
	b := start(body)
	return len(b) > 0 && b[0] == '<'
}

// isDocument reports whether markup is a whole page rather than a fragment.
func isDocument(body []byte) bool {
	head := bytes.ToLower(start(body)[:min(len(start(body)), 8192)])
	for _, p := range [][]byte{[]byte("<!doctype"), []byte("<?xml")} {
		if bytes.HasPrefix(head, p) {
			return true
		}
	}
	for _, t := range [][]byte{[]byte("<html"), []byte("<head"), []byte("<body")} {
		if bytes.Contains(head, t) {
			return true
		}
	}
	return false
}

// fragmentContext is the element a fragment is parsed in: table rows must
// be parsed inside a table, or the parser drops their tags.
func fragmentContext(body []byte) *html.Node {
	b := bytes.ToLower(start(body))
	ctx := atom.Body
	for _, c := range []struct {
		tag string
		in  atom.Atom
	}{
		{"<tr", atom.Tbody}, {"<td", atom.Tr}, {"<th", atom.Tr}, {"<tbody", atom.Table},
		{"<thead", atom.Table}, {"<tfoot", atom.Table}, {"<caption", atom.Table}, {"<colgroup", atom.Table},
		{"<col", atom.Colgroup}, {"<option", atom.Select}, {"<optgroup", atom.Select},
	} {
		if bytes.HasPrefix(b, []byte(c.tag)) && len(b) > len(c.tag) && !isNameByte(b[len(c.tag)]) {
			ctx = c.in
			break
		}
	}
	return &html.Node{Type: html.ElementNode, Data: ctx.String(), DataAtom: ctx}
}

func isNameByte(c byte) bool { return c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' }

// renderFragment applies the steps to a fragment and serialises only it.
func (p *Pipeline) renderFragment(body []byte, ctx *Context) ([]byte, error) {
	doc, err := html.Parse(bytes.NewReader(nil)) // html, head and body for the steps
	if err != nil {
		return nil, err
	}
	var holder *html.Node
	Walk(doc, func(n *html.Node) bool {
		if n.DataAtom == atom.Body {
			holder = n
		}
		return holder == nil
	})
	context := fragmentContext(body)
	nodes, err := html.ParseFragment(bytes.NewReader(body), context)
	if err != nil {
		return nil, err
	}
	// The nodes keep their parsing context only while they are parsed.
	wrap := &html.Node{Type: html.ElementNode, Data: context.Data, DataAtom: context.DataAtom}
	holder.AppendChild(wrap)
	for _, n := range nodes {
		wrap.AppendChild(n)
	}
	declareUTF8(doc)
	for _, s := range p.steps {
		if err := s.Apply(doc, ctx); err != nil {
			return nil, err
		}
	}
	var out bytes.Buffer
	for c := wrap.FirstChild; c != nil; c = c.NextSibling {
		if err := html.Render(io.Writer(&out), c); err != nil {
			return nil, err
		}
	}
	return out.Bytes(), nil
}
