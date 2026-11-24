package securityscan

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

func finding(id, title, url, param string, cwe ...string) store.SecurityFinding {
	f := store.SecurityFinding{RemoteID: id, Title: title, Severity: "high", CWE: cwe}
	if url != "" {
		f.URL = &url
	}
	if param != "" {
		f.Parameter = &param
	}
	return f
}

func TestHarden(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal([]byte(doc0), &doc); err != nil {
		t.Fatal(err)
	}
	m, _ := ParseMap([]byte(doc0))
	post := finding("xss2", "Reflected XSS in comment", "https://site/wp-comments-post.php", "comment")
	method := "POST"
	post.HTTPMethod = &method
	fixes := Harden(doc, m, []store.SecurityFinding{
		finding("sqli", "SQL error", "https://site/?s=1'", "s", "CWE-89"),
		finding("xss", "Reflected cross-site scripting", "https://site/contact/", "", ""), // no input named
		post,
		finding("typed", "SQL injection", "https://site/blog/x/", "slug", "CWE-89"),     // typed: precise, left alone
		finding("area", "SQL injection", "https://site/wp-admin/x.php", "id", "CWE-89"), // the area's
		finding("hdr", "CSP missing", "https://site/", ""),
		finding("other", "Open redirect", "https://site/?s=x", "s", "CWE-601"), // no rule for it
	})
	got := map[string]string{}
	for _, f := range fixes {
		got[f.Finding] = f.Change
	}
	if len(got) != 2 || got["sqli"] != "search.s refuses SQL characters" || got["xss2"] != "comment.comment refuses markup" {
		t.Fatalf("fixes: %v", got)
	}
	b, _ := json.Marshal(doc)
	if !strings.Contains(string(b), `"s":{"max_length":200,"rules":["no_sql_meta"],"type":"string"}`) {
		t.Fatalf("rule not in the map: %s", b)
	}
	// once there, the same finding changes nothing
	if again := Harden(doc, m, []store.SecurityFinding{finding("sqli", "SQL error", "https://site/?s=1'", "s", "CWE-89")}); len(again) != 0 {
		t.Fatalf("again: %v", again)
	}
}

const doc0 = `{
 "types":{"slug":{"base":"string","pattern":"^[a-z-]+$"}},
 "routes":[
  {"id":"post","path":"/blog/{slug}/","method":"GET","params":{"slug":{"type":"slug"}}},
  {"id":"search","path":"/","method":"GET","query":{"s":{"type":"string","max_length":200}}}],
 "forms":[{"id":"comment","route":"/wp-comments-post.php","fields":{"comment":{"type":"text"}}}],
 "areas":[{"id":"site","prefix":"/"}]}`
