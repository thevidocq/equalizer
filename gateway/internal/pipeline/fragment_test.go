package pipeline_test

import (
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/pipeline"
)

// AJAX answers are fragments: they come back as fragments, links rewritten.
func TestFragments(t *testing.T) {
	for what, c := range map[string][2]string{
		"list":       {`<ul><li><a href="/detail.jsp?id=7">Permit</a></li></ul>`, `<ul><li><a href="/permits/7">Permit</a></li></ul>`},
		"table rows": {`<tr id="post-1"><td><a href="/detail.jsp?id=1">Edit</a></td></tr>`, `<tr id="post-1"><td><a href="/permits/1">Edit</a></td></tr>`},
		"cells":      {`<td>a</td><td>b</td>`, `<td>a</td><td>b</td>`},
		"options":    {` <option value="1">One</option><option value="2">Two</option>`, ` <option value="1">One</option><option value="2">Two</option>`},
		"text first": {`<b>Saved.</b> <a href="/index.jsp">Back</a>`, `<b>Saved.</b> <a href="/">Back</a>`},
	} {
		got, _ := render(t, "text/html", c[0])
		if got != c[1] {
			t.Errorf("%s:\n got %s\nwant %s", what, got, c[1])
		}
	}
	// A page is still a page.
	got, _ := render(t, "text/html", "<!DOCTYPE html><title>x</title><p>y")
	if got != `<!DOCTYPE html><html><head><meta charset="utf-8"/><title>x</title></head><body><p>y</p></body></html>` &&
		got != `<!DOCTYPE html><html><head><title>x</title></head><body><p>y</p></body></html>` {
		t.Errorf("page: %s", got)
	}
}

func TestMarkup(t *testing.T) {
	for body, want := range map[string]bool{
		"[]": false, `{"ok":true}`: false, "Saved.": false, "": false,
		"<div>x</div>": true, "\xef\xbb\xbf\n  <p>x": true, "<!DOCTYPE html>": true,
	} {
		if pipeline.Markup([]byte(body)) != want {
			t.Errorf("%q: %v", body, !want)
		}
	}
}
