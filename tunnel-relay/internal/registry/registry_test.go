package registry

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRegistry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "connectors.json")
	write := func(body string) {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"connectors":[{"id":"cn-1","tenant":"t","app":"a","token_sha256":"` + Hash("secret") + `"}]}`)
	r := New(path)
	if changed, err := r.Reload(); err != nil || !changed {
		t.Fatalf("reload: %v %v", changed, err)
	}
	if c, ok := r.Authenticate("cn-1", "secret"); !ok || c.Tenant != "t" {
		t.Fatal("valid token rejected")
	}
	for _, c := range [][2]string{{"cn-1", "wrong"}, {"cn-1", ""}, {"cn-2", "secret"}} {
		if _, ok := r.Authenticate(c[0], c[1]); ok {
			t.Errorf("%v accepted", c)
		}
	}
	if changed, _ := r.Reload(); changed {
		t.Error("unchanged file reported as changed")
	}

	// A broken file keeps the previous set.
	time.Sleep(10 * time.Millisecond)
	write(`{"connectors":[{"id":"BAD ID","token_sha256":"00"}]}`)
	if _, err := r.Reload(); err == nil {
		t.Fatal("invalid file accepted")
	}
	if _, ok := r.Authenticate("cn-1", "secret"); !ok {
		t.Fatal("previous set lost after a bad reload")
	}
}

func TestParseRejects(t *testing.T) {
	for _, body := range []string{
		`not json`,
		`{"connectors":[{"id":"cn-1","token_sha256":"abc"}]}`,
		`{"connectors":[{"id":"cn-1","token_sha256":"` + Hash("a") + `"},{"id":"cn-1","token_sha256":"` + Hash("b") + `"}]}`,
	} {
		if _, err := Parse([]byte(body)); err == nil {
			t.Errorf("accepted: %s", body)
		}
	}
}
