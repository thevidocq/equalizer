package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// The CMS version stays readable however long the report's lists are (it
// was cut off: the keys came alphabetically and the lists filled the space).
func TestReportKeepsFacts(t *testing.T) {
	var long []string
	for i := 0; i < 2000; i++ {
		long = append(long, fmt.Sprintf("/page-%d-with-a-long-name", i))
	}
	rep, _ := json.Marshal(map[string]any{"about": map[string]any{"discovery": 7},
		"report": map[string]any{"cms": "wordpress", "version": "5.7.22", "kept_out": long, "routes_added": long,
			"components": []any{map[string]any{"name": "plugin:elementor", "version": "4.0.2"}}, "pages": long}})
	out := compactReport(rep)
	for _, want := range []string{`version: "5.7.22"`, `cms: "wordpress"`, "plugin:elementor", "kept_out (2000)", "…and 1960 more"} {
		if !strings.Contains(out, want) {
			t.Fatalf("%q missing from the report the model reads:\n%s", want, out[:min(len(out), 600)])
		}
	}
	if strings.Contains(out, "pages (") {
		t.Fatal("the page list is sent")
	}
}

func TestGuide(t *testing.T) {
	if out := Guide(""); !strings.Contains(out, "Sections of the guide") || !strings.Contains(out, "The assistant") {
		t.Fatalf("contents: %s", out[:min(len(out), 300)])
	}
	if out := Guide("test accounts verification check"); !strings.Contains(out, "Check on the site") {
		t.Fatalf("no section about the check: %s", out[:min(len(out), 300)])
	}
	if out := Guide("zzzz qqqq"); !strings.Contains(out, "Nothing in the guide") {
		t.Fatal(out)
	}
}

type siteSrc struct{ src }

func (siteSrc) Status(context.Context) (any, error) {
	return map[string]any{"connector": "online", "certificate_expires": "2027-01-02", "error": "dial tcp 203.0.113.5:443 for jane@acme.test"}, nil
}
func (siteSrc) Versions(context.Context) (any, error) {
	return []any{map[string]any{"version": 6, "status": "in_review"}}, nil
}
func (siteSrc) Verification(_ context.Context, v int) (any, error) {
	return map[string]any{"version": v, "score": "34/34"}, nil
}
func (siteSrc) Compare(_ context.Context, a, b int) (any, error) {
	return map[string]any{"changes": []any{fmt.Sprintf("%d→%d", a, b)}}, nil
}

func (siteSrc) Security(context.Context) (any, error) {
	return map[string]any{"targets": []any{map[string]any{"target": "protected", "findings": []any{
		map[string]any{"title": "SQL error", "severity": "high", "evidence": "from 198.51.100.7", "map": map[string]any{"verdict": "free_text"}}}}}}, nil
}

func TestSiteTools(t *testing.T) {
	var ans Answer
	run := func(name, in string) string {
		return Exec(context.Background(), &siteSrc{}, NewMasker(nil), Call{Name: name, Input: json.RawMessage(in)}, &ans)
	}
	for name, want := range map[string]string{
		"get_site": "online", "list_versions": "in_review", "get_check": `"version":0`, "read_guide": "Sections",
		"get_security": "free_text",
	} {
		if out := run(name, `{}`); !strings.Contains(out, want) {
			t.Fatalf("%s: %s", name, out)
		}
	}
	if out := run("get_site", `{}`); strings.Contains(out, "203.0.113.5") || strings.Contains(out, "jane@") {
		t.Fatalf("unmasked: %s", out)
	}
	if out := run("get_security", `{}`); strings.Contains(out, "198.51.100.7") {
		t.Fatalf("evidence unmasked: %s", out)
	}
	if out := run("compare_versions", `{"from":5,"to":6}`); !strings.Contains(out, "5→6") {
		t.Fatal(out)
	}
	if out := run("compare_versions", `{"from":5}`); !strings.HasPrefix(out, "error") {
		t.Fatal(out)
	}
	// a source without them says so
	if out := Exec(context.Background(), &src{}, NewMasker(nil), Call{Name: "get_site", Input: json.RawMessage(`{}`)}, &ans); !strings.HasPrefix(out, "error") {
		t.Fatal(out)
	}
}
