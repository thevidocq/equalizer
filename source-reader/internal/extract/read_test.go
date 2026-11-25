package extract

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "rewrite schema/source/examples/wordpress.json")

// site copies the fixture and adds what git cannot hold: a .git folder,
// and a .env file the repository ignores.
func site(t *testing.T) string {
	dir := filepath.Join(t.TempDir(), "wp")
	err := filepath.Walk("testdata/wp", func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		to := filepath.Join(dir, strings.TrimPrefix(p, "testdata/wp"))
		if info.IsDir() {
			return os.MkdirAll(to, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(to, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	extra := map[string]string{".env": "DB_PASSWORD=secretpw-never-read\n", ".git/hook.php": "<?php $_GET[\"in_git\"];\n"}
	for name, body := range extra {
		_ = os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestReadWordPress(t *testing.T) {
	s, err := Read(site(t), time.Date(2026, 11, 24, 16, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	out := string(b)
	// never a secret, an upload or version control
	for _, never := range []string{"secretpw", "from_config", "uploaded", "in_git"} {
		if strings.Contains(out, never) {
			t.Fatalf("%q in the summary", never)
		}
	}
	if s.Stack != "wordpress" {
		t.Fatalf("stack %s", s.Stack)
	}
	routes := map[string]Route{}
	for _, r := range s.Routes {
		routes[r.Origin+" "+r.Path+" "+r.Action] = r
	}
	for _, want := range []string{
		"rest_route /wp-json/demo/v1/items/{id} ", "rest_route /wp-json/demo/v1/items ",
		"wp_ajax_nopriv /wp-admin/admin-ajax.php demo_search", "wp_ajax /wp-admin/admin-ajax.php demo_save",
		"admin_post /wp-admin/admin-post.php demo_export", "php_file /wp-content/plugins/demo/direct.php ",
	} {
		if _, ok := routes[want]; !ok {
			t.Errorf("route %q missing: %v", want, keys(routes))
		}
	}
	if r := routes["rest_route /wp-json/demo/v1/items/{id} "]; !r.Public || r.Params["id"] != "int" || r.Methods[0] != "GET" {
		t.Errorf("item route: %+v", r)
	}
	if r := routes["rest_route /wp-json/demo/v1/items "]; r.Public || r.Methods[0] != "POST" {
		t.Errorf("create route: %+v", r)
	}
	if _, ok := routes["php_file /wp-content/plugins/demo/guarded.php "]; ok {
		t.Error("a guarded file is no entry point")
	}
	types := map[string]string{}
	for _, in := range s.Inputs {
		types[in.Source+":"+in.Name] = in.Type
	}
	for k, v := range map[string]string{"query:id": "int", "body:q": "text", "body:email": "email", "query:order": "enum", "query:raw": "unknown", "query:name": "unknown"} {
		if types[k] != v {
			t.Errorf("input %s: %q, want %q (%v)", k, types[k], v, types)
		}
	}
	risks := map[string]string{}
	for _, r := range s.Risks {
		risks[r.Kind] = r.Input
	}
	if risks["sql"] != "raw" || risks["include"] != "page" {
		t.Errorf("risks: %+v", s.Risks)
	}
	comps := map[string]string{}
	for _, c := range s.Components {
		comps[c.Name] = *c.Version
	}
	if comps["wordpress"] != "6.4.2" || comps["plugin:demo"] != "1.2.3" || comps["theme:twentyx"] != "2.0" || comps["library:guzzlehttp/guzzle"] != "7.8.1" {
		t.Errorf("components: %v", comps)
	}
	example := filepath.Join("..", "..", "..", "schema", "source", "examples", "wordpress.json")
	if *update {
		if err := os.WriteFile(example, append(b, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(example)
	if err != nil || strings.TrimSpace(string(want)) != out {
		t.Fatalf("the summary differs from %s (go test ./internal/extract -update to rewrite it)", example)
	}
}

func keys(m map[string]Route) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

// A FIFO named like code never blocks the read.
func TestFIFO(t *testing.T) {
	dir := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(dir, "x.php"), 0o600); err != nil {
		t.Skip(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "y.php"), []byte("<?php echo $_GET['q'];"), 0o600)
	done := make(chan *Summary, 1)
	go func() { s, _ := Read(dir, time.Now()); done <- s }()
	select {
	case s := <-done:
		if s.Files.Read != 1 {
			t.Fatalf("%+v", s.Files)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the read blocked on a FIFO")
	}
}
