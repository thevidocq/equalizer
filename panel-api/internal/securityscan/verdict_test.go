package securityscan

import (
	"strings"
	"testing"

	"github.com/thevidocq/equalizer/panel-api/internal/cveradar"
)

// A map as the agent writes them: forms post to a path, the whole site is
// an area, /wp-admin/ an area of its own that only reads and posts.
const doc = `{
 "types":{"slug":{"base":"string","pattern":"^[a-z-]+$"},"id":{"base":"int"}},
 "routes":[
  {"id":"post","path":"/blog/{slug}/","method":"GET","params":{"slug":{"type":"slug"}},"query":{"replytocom":{"type":"id"}}},
  {"id":"search","path":"/","method":"GET","query":{"s":{"type":"string","max_length":200}}},
  {"id":"page","path":"/{slug}/","method":"GET","params":{"slug":{"type":"slug"}}},
  {"id":"invoice","path":"/invoices/{id}","method":"GET","params":{"id":{"type":"id"}},"authz":{"owner":"x"}}],
 "forms":[{"id":"comment","route":"/wp-comments-post.php","fields":{"comment":{"type":"text"},"email":{"type":"email"}}}],
 "api":[{"id":"wp_json","path":"/wp-json/wp/v2/posts/{id}","method":"GET","params":{"id":{"type":"id"}}}],
 "areas":[{"id":"site","prefix":"/"},{"id":"admin","prefix":"/wp-admin/","methods":["GET","POST"]}],
 "assets":[{"prefix":"/wp-content/uploads/","types":["image/png"]}]}`

func TestJudge(t *testing.T) {
	m, err := ParseMap([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ url, method, param, want, entry string }{
		{"", "", "", Server, ""},
		{"https://site/blog/hello/", "GET", "slug", Precise, "post"},
		{"https://site/blog/hello", "HEAD", "replytocom", Precise, "post"}, // HEAD is GET, the final slash does not count
		{"https://site//blog/hello/;x", "GET", "", Precise, "post"},        // as the legacy server reads the path
		{"https://site/blog/hello/?utm=1", "GET", "utm", Area, "site"},     // a key the route does not declare: the area's
		{"https://site/blog/Hello_9/", "GET", "slug", Area, "site"},        // a value that does not fit: the area's
		{"https://site/blog/a'b'/", "GET", "slug", Refused, "post"},        // unless the gateway refuses it on the way
		{"https://site/?s=x", "GET", "s", FreeText, "search"},
		{"https://site/wp-login.php", "GET", "", Area, "site"}, // /{slug}/ shape, not its value
		{"https://site/about/", "GET", "", Precise, "page"},
		{"https://site/wp-comments-post.php", "POST", "comment", FreeText, "comment"},
		{"https://site/wp-comments-post.php", "POST", "email", Precise, "comment"},
		{"https://site/wp-comments-post.php", "POST", "other", Refused, "comment"}, // undeclared: deny by default
		{"https://site/wp-json/wp/v2/posts/7", "GET", "id", Precise, "wp_json"},
		{"https://site/wp-json/wp/v2/posts/x", "GET", "id", Area, "site"},
		{"https://site/invoices/abc", "GET", "", Refused, "invoice"}, // authz entries are never bypassed
		{"https://site/wp-admin/options.php", "POST", "", Area, "admin"},
		{"https://site/wp-admin/options.php", "DELETE", "", Refused, ""}, // the longest area decides the methods
		{"https://site/wp-admin", "POST", "", Area, "admin"},             // "/x" falls in "/x/"
		{"https://site/wp-admin", "GET", "", Precise, "page"},            // and has the shape and value of /{slug}/
		{"https://site/wp-content/uploads/a.png", "GET", "", Asset, ""},
		{"https://site/wp-content/uploads/", "GET", "", Area, "site"}, // a prefix alone is no file
		{"https://site/.git/config", "GET", "", Area, "site"},
		{"https://site/blog/hello/", "", "", Precise, "post"}, // no method: GET
	} {
		v := m.Judge(c.url, c.method, c.param)
		entry := ""
		if v.Entry != nil {
			entry = *v.Entry
		}
		if v.Verdict != c.want || entry != c.entry {
			t.Errorf("%s %s %s: %s %q, want %s %q", c.method, c.url, c.param, v.Verdict, entry, c.want, c.entry)
		}
	}
	noArea, _ := ParseMap([]byte(strings.Replace(doc, `"areas":[{"id":"site","prefix":"/"},`, `"areas":[`, 1)))
	for _, u := range []string{"https://site/.git/config", "https://site/blog/hello/?utm=1"} {
		if v := noArea.Judge(u, "GET", "utm"); v.Verdict != Refused {
			t.Errorf("without an area %s: %s", u, v.Verdict)
		}
	}
	var none *Map
	if v := none.Judge("https://site/", "GET", ""); v.Verdict != Refused {
		t.Errorf("no map: %v", v.Verdict)
	}
}

func TestFindingIsCleaned(t *testing.T) {
	long := strings.Repeat("é", 5000)
	nul := "a\x00b"
	f := Finding(cveradar.Vulnerability{ID: "x", Title: "t\x00", Severity: "HIGH", Evidence: &long, URL: &nul, CVE: make([]string, 50)})
	if f.Severity != "high" || len([]rune(*f.Evidence)) > 2001 || len(f.CVE) != 20 || f.Title != "t" || *f.URL != "ab" {
		t.Fatalf("%+v", f)
	}
	if Finding(cveradar.Vulnerability{Severity: "nonsense"}).Severity != "info" {
		t.Fatal("unknown severity")
	}
}
