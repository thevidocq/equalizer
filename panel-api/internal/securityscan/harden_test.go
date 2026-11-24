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
		finding("sqli2", "Blind SQL injection", "https://site/?s=2", "s"),                 // the same input: in the same draft
		finding("xss", "Reflected cross-site scripting", "https://site/contact/", "", ""), // no input named
		post,
		finding("typed", "SQL injection", "https://site/blog/x/", "slug", "CWE-89"),     // typed: precise, left alone
		finding("area", "SQL injection", "https://site/wp-admin/x.php", "id", "CWE-89"), // the area's
		finding("hdr", "CSP missing", "https://site/", ""),
		finding("other", "Open redirect", "https://site/?s=x", "s", "CWE-601"), // no rule for it
	})
	got := map[string]string{}
	for _, f := range fixes.Added {
		got[f.Finding] = f.Change
	}
	if len(got) != 3 || got["sqli"] != "search.s refuses SQL characters" || got["sqli2"] != "search.s refuses SQL characters" ||
		got["xss2"] != "comment.comment refuses markup" || len(fixes.Already) != 0 {
		t.Fatalf("fixes: %v %v", got, fixes.Already)
	}
	b, _ := json.Marshal(doc)
	if !strings.Contains(string(b), `"s":{"max_length":200,"rules":["no_sql_meta"],"type":"string"}`) {
		t.Fatalf("rule not in the map: %s", b)
	}
	// once there, the same finding is hardened already
	again := Harden(doc, m, []store.SecurityFinding{finding("sqli", "SQL error", "https://site/?s=1'", "s", "CWE-89")})
	if len(again.Added) != 0 || len(again.Already) != 1 {
		t.Fatalf("again: %+v", again)
	}
}

// Words, not substrings, and no rule for code injection: no text rule stops eval.
func TestRulesFor(t *testing.T) {
	for title, want := range map[string]string{
		"SQLite version disclosure":      "",
		"Blind SQLi in id":               "no_sql_meta",
		"Reflected Cross-Site Scripting": "no_html",
		"PHP code injection (eval)":      "",
		"Server-side template injection": "no_template_meta",
	} {
		got := strings.Join(rules(store.SecurityFinding{Title: title}), ",")
		if got != want {
			t.Errorf("%q: %q, want %q", title, got, want)
		}
	}
	if got := rules(store.SecurityFinding{Title: "x", CWE: []string{"CWE-94"}}); len(got) != 0 {
		t.Errorf("CWE-94: %v", got)
	}
}

const doc0 = `{
 "types":{"slug":{"base":"string","pattern":"^[a-z-]+$"}},
 "routes":[
  {"id":"post","path":"/blog/{slug}/","method":"GET","params":{"slug":{"type":"slug"}}},
  {"id":"search","path":"/","method":"GET","query":{"s":{"type":"string","max_length":200}}}],
 "forms":[{"id":"comment","route":"/wp-comments-post.php","fields":{"comment":{"type":"text"}}}],
 "areas":[{"id":"site","prefix":"/"}]}`
