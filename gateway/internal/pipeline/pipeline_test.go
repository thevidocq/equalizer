package pipeline_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/thevidocq/equalizer/gateway/internal/legacyurl"
	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
	"github.com/thevidocq/equalizer/gateway/internal/pipeline"
	"github.com/thevidocq/equalizer/gateway/internal/pipeline/clean"
	"github.com/thevidocq/equalizer/gateway/internal/pipeline/forms"
	"github.com/thevidocq/equalizer/gateway/internal/pipeline/rewrite"
)

func render(t *testing.T, contentType, page string) (string, pipeline.Stats) {
	t.Helper()
	base, _ := url.Parse("http://10.20.0.15:8080/")
	m := &mapmodel.Model{
		Routes: []mapmodel.Route{
			{ID: "home", Path: "/", LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/index.jsp"}},
			{ID: "detail", Path: "/permits/{id}", Params: map[string]mapmodel.FieldSpec{"id": {Type: "int"}},
				LegacyRequest: mapmodel.LegacyRequest{Method: "GET", Path: "/detail.jsp", Query: map[string]string{"id": "{{params.id}}"}}},
		},
		Assets: []mapmodel.Asset{{Prefix: "/css/", Types: []string{"text/css"}}, {Prefix: "/img/", Types: []string{"image/png"}}},
	}
	rw, err := legacyurl.New(base, m)
	if err != nil {
		t.Fatal(err)
	}
	pageURL, _ := url.Parse("http://10.20.0.15:8080/index.jsp")
	ctx := &pipeline.Context{PageURL: pageURL, URLs: rw}
	out, err := pipeline.New(clean.Step{}, rewrite.Step{}).Render([]byte(page), contentType, ctx)
	if err != nil {
		t.Fatal(err)
	}
	return string(out), ctx.Stats
}

func TestFaithfulRendering(t *testing.T) {
	page := `<html><head><meta name="generator" content="Liferay 6.0.6">
<link rel="stylesheet" href="css/style.css"><base href="http://10.20.0.15:8080/">
</head><body><!-- build 2009 query=SELECT * FROM users -->
<a href="detail.jsp?id=42">permit</a>
<a href="http://10.20.0.15:8080/admin/console.jsp">admin</a>
<a href="https://www.example.org/">pay</a>
<a href="#top">up</a><a href="javascript:void(0)">js</a>
<img src="/img/logo.png" srcset="/img/logo.png 1x, http://10.20.0.15:8080/img/logo@2x.png 2x">
<form action="http://10.20.0.15:8080/detail.jsp?id=7" method="get"></form>
</body></html>`
	out, stats := render(t, "text/html", page)

	mustContain := []string{
		`href="/css/style.css"`,
		`href="/permits/42"`,
		`href="/admin/console.jsp"`, // unmapped: relative, then 404 from the gateway
		`href="https://www.example.org/"`,
		`href="#top"`, `href="javascript:void(0)"`,
		`srcset="/img/logo.png 1x, /img/logo@2x.png 2x"`,
		`action="/permits/7"`,
	}
	for _, s := range mustContain {
		if !strings.Contains(out, s) {
			t.Errorf("missing %s in:\n%s", s, out)
		}
	}
	for _, s := range []string{"10.20.0.15", "Liferay", "SELECT", "<base", "<!--"} {
		if strings.Contains(out, s) {
			t.Errorf("the page still contains %q:\n%s", s, out)
		}
	}
	if stats.Unmapped != 1 || stats.UnmappedPaths[0] != "/admin/console.jsp" || stats.Rewritten < 5 {
		t.Errorf("unexpected stats: %+v", stats)
	}
}

func TestCharsetConversion(t *testing.T) {
	page := "<html><head><meta http-equiv=\"Content-Type\" content=\"text/html; charset=iso-8859-1\"></head>" +
		"<body><p>caf\xe9 &egrave; \x80</p></body></html>"
	out, stats := render(t, "text/html; charset=ISO-8859-1", page)
	if stats.Charset != "iso-8859-1" {
		t.Errorf("detected encoding %q", stats.Charset)
	}
	if !strings.Contains(out, "café è") || !strings.Contains(out, `content="text/html; charset=utf-8"`) {
		t.Errorf("wrong conversion:\n%q", out)
	}
}

type fakeForms struct{}

func (fakeForms) Match(action *url.URL, hidden map[string]string) (pipeline.FormMatch, bool) {
	if action.Path != "/supplement.do" || hidden["permitId"] == "" {
		return pipeline.FormMatch{}, false
	}
	return pipeline.FormMatch{FormID: "supplement", Action: "/permits/" + hidden["permitId"] + "/supplement",
		Hidden: map[string]string{"vs": hidden["vs"]}, Rename: map[string]string{"text": "message"}}, true
}

func (fakeForms) Token(m pipeline.FormMatch, cs string) (string, error) {
	return "TOKEN-" + m.FormID + "-" + cs, nil
}

func TestFormBinding(t *testing.T) {
	base, _ := url.Parse("http://10.20.0.15:8080/")
	rw, _ := legacyurl.New(base, &mapmodel.Model{})
	pageURL, _ := url.Parse("http://10.20.0.15:8080/detail.jsp?id=42")
	page := `<html><body>
<form method="POST" action="supplement.do" accept-charset="ISO-8859-1">
<input type="hidden" name="permitId" value="42"><input type="hidden" name="vs" value="internal-state">
<input type="hidden" name="debug" value="1">
<label for="t">Text</label><textarea id="t" name="text"></textarea>
<button type="submit" formaction="other.do">Send</button></form>
<form method="post" action="admin.do"><input type="hidden" name="role" value="admin"><input name="q"></form>
<form method="get" action="search.jsp"><input type="hidden" name="lang" value="en"><input name="q"></form>
</body></html>`
	ctx := &pipeline.Context{PageURL: pageURL, URLs: rw, Forms: fakeForms{}}
	out, err := pipeline.New(forms.Step{}, rewrite.Step{}).Render([]byte(page), "text/html; charset=iso-8859-1", ctx)
	if err != nil {
		t.Fatal(err)
	}
	body := string(out)
	for _, want := range []string{
		`action="/permits/42/supplement"`,
		`name="_eq_token" value="TOKEN-supplement-iso-8859-1"`,
		`<textarea id="t" name="message">`,
		`<label for="t">Text</label>`,
		`name="lang"`, // GET forms keep their hidden inputs
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in\n%s", want, body)
		}
	}
	for _, banned := range []string{"internal-state", "permitId", `name="debug"`, "formaction", "accept-charset", `value="admin"`} {
		if strings.Contains(body, banned) {
			t.Errorf("%q must not reach the page:\n%s", banned, body)
		}
	}
	if ctx.Stats.Forms != 1 || ctx.Stats.FormsUnmapped != 1 {
		t.Errorf("stats: %+v", ctx.Stats)
	}
}
