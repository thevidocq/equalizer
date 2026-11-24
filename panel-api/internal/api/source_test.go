package api_test

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
)

// readerCall is the source reader calling the panel with its token.
func readerCall(t *testing.T, e *env, token, path string, body any, out any) int {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", e.srv.URL+path, strings.NewReader(string(b)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil && resp.StatusCode == 200 {
		_ = json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode
}

type sourceView struct {
	Configured  bool
	Method      *string
	HasPassword bool `json:"has_password"`
	Status      string
	PublicKey   *string `json:"public_key"`
	LastRead    *struct {
		ID     int64
		Status string
		Error  *string
		Counts *struct{ Routes, Inputs, Risks int }
	} `json:"last_read"`
}

func TestSourceReader(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	boss := e.join("boss@acme.test")
	boss.must(201, "POST", "/v1/tenants/acme/invitations", map[string]string{"email": "ap@acme.test", "name": "Ap", "role": "viewer"}, nil)
	viewer := e.join("ap@acme.test")
	boss.must(201, "POST", "/v1/tenants/acme/connectors", map[string]string{"id": "cn-web01"}, nil)
	boss.must(201, "POST", "/v1/tenants/acme/apps", map[string]any{"id": "blog", "name": "Blog", "public_domain": "blog.acme.test", "kind": "wordpress"}, nil)
	boss.must(201, "POST", "/v1/tenants/acme/apps", map[string]any{"id": "shop", "name": "Shop", "public_domain": "shop.acme.test"}, nil)
	start := map[string]any{"connector_id": "cn-web01", "address": "127.0.0.1:8443", "source": true}

	var v sourceView
	boss.must(200, "GET", "/v1/apps/blog/source", nil, &v)
	if v.Configured || v.Status != "not_set_up" || v.LastRead != nil {
		t.Fatalf("empty: %+v", v)
	}
	// Not ready: neither a read nor a discovery that asks for the source.
	var apiErr struct{ Error string }
	boss.must(409, "POST", "/v1/apps/blog/source/reads", nil, &apiErr)
	boss.must(409, "POST", "/v1/apps/blog/discoveries", start, nil)

	// Settings: administrators only, checked, the password never returned.
	viewer.must(403, "PUT", "/v1/apps/blog/source", map[string]any{"method": "local", "path": "/"}, nil)
	for _, bad := range []map[string]any{
		{"method": "ftp", "path": "/var/www"},
		{"method": "sftp", "host": "web; id", "username": "reader", "path": "/var/www"},
		{"method": "sftp", "host": "web", "username": "-oProxyCommand=x", "path": "/var/www"},
		{"method": "sftp", "host": "web", "username": "reader", "path": "/var/www/$(id)"},
		{"method": "sftp", "host": "web", "username": "reader", "path": "/var/../etc"},
		{"method": "sftp", "host": "web", "username": "reader", "path": "/var/www;id"},
		{"method": "sftp", "host": "-oProxyCommand=id", "username": "reader", "path": "/var/www"},
		{"method": "smb", "host": "fs", "username": "reader", "share": "www", "path": "/", "password": "a\nb"},
		{"method": "smb", "host": "fs", "username": "reader", "share": "a/b", "path": "/"},
	} {
		boss.must(422, "PUT", "/v1/apps/blog/source", bad, nil)
	}
	boss.must(200, "PUT", "/v1/apps/blog/source", map[string]any{"method": "smb", "host": "fs.lan", "username": "reader",
		"share": "www", "path": "/blog", "password": "smb-secret-pw"}, &v)
	if !v.Configured || !v.HasPassword || v.Status != "not_installed" {
		t.Fatalf("smb: %+v", v)
	}
	var raw map[string]any
	boss.must(200, "GET", "/v1/apps/blog/source", nil, &raw)
	if b, _ := json.Marshal(raw); strings.Contains(string(b), "smb-secret") {
		t.Fatalf("password returned: %s", b)
	}
	// Another host with no password: the saved one is not carried there.
	boss.must(200, "PUT", "/v1/apps/blog/source", map[string]any{"method": "smb", "host": "other.lan", "username": "reader",
		"share": "www", "path": "/blog"}, &v)
	if v.HasPassword {
		t.Fatal("the password followed another host")
	}
	boss.must(200, "PUT", "/v1/apps/blog/source", map[string]any{"method": "sftp", "host": "web01.lan", "username": "reader",
		"path": "/var/www/html"}, &v)

	// The token: shown once with the install command; the reader says hello.
	var tok struct{ Token, Install string }
	viewer.must(403, "POST", "/v1/apps/blog/source/token", nil, nil)
	boss.must(201, "POST", "/v1/apps/blog/source/token", nil, &tok)
	if !strings.Contains(tok.Install, "EQ_SOURCE_TOKEN="+tok.Token) || !strings.Contains(tok.Install, "EQ_PANEL_URL=https://panel.test") ||
		!strings.Contains(tok.Install, "equalizer-source-reader:latest") {
		t.Fatalf("install: %s", tok.Install)
	}
	if c := readerCall(t, e, "wrong-token-0123456789abc", "/v1/source/hello", map[string]any{"version": "0.2.29"}, nil); c != 401 {
		t.Fatal("a wrong token")
	}
	if readerCall(t, e, tok.Token, "/v1/source/hello", map[string]any{"version": "0.2.29", "public_key": "ssh-rsa AAAA"}, nil) != 422 {
		t.Fatal("not an ed25519 key")
	}
	key := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIA reader@equalizer"
	if readerCall(t, e, tok.Token, "/v1/source/hello", map[string]any{"version": "0.2.29", "public_key": key}, nil) != 204 {
		t.Fatal("hello")
	}
	boss.must(200, "GET", "/v1/apps/blog/source", nil, &v)
	if v.Status != "ready" || v.PublicKey == nil || *v.PublicKey != key {
		t.Fatalf("ready: %+v", v)
	}
	if readerCall(t, e, tok.Token, "/v1/source/claim", nil, nil) != 204 {
		t.Fatal("nothing queued")
	}

	// A site in Docker: the command takes its container's volumes, read only;
	// a folder of the host is mounted at the same path.
	boss.must(422, "PUT", "/v1/apps/blog/source", map[string]any{"method": "local", "path": "/var/www/html", "container": "wp; rm"}, nil)
	boss.must(200, "PUT", "/v1/apps/blog/source", map[string]any{"method": "local", "path": "/var/www/html", "container": "wp"}, nil)
	var dock struct{ Token, Install string }
	boss.must(201, "POST", "/v1/apps/blog/source/token", nil, &dock)
	if !strings.Contains(dock.Install, "--volumes-from wp:ro") || !strings.Contains(dock.Install, "EQ_SOURCE_SITE=/ ") {
		t.Fatalf("docker install: %s", dock.Install)
	}
	for _, p := range []string{"/", "/etc", "/root/site", "/var", "/proc/1"} {
		boss.must(422, "PUT", "/v1/apps/blog/source", map[string]any{"method": "local", "path": p}, nil)
	}
	boss.must(200, "PUT", "/v1/apps/blog/source", map[string]any{"method": "local", "path": "/srv/my blog"}, nil)
	boss.must(201, "POST", "/v1/apps/blog/source/token", nil, &dock)
	if !strings.Contains(dock.Install, "-v '/srv/my blog:/srv/my blog:ro'") || strings.Contains(dock.Install, "volumes-from") {
		t.Fatalf("folder install: %s", dock.Install)
	}
	tok.Token = dock.Token
	if readerCall(t, e, tok.Token, "/v1/source/hello", map[string]any{"version": "0.2.30", "public_key": key}, nil) != 204 {
		t.Fatal("hello with the new token")
	}
	boss.must(200, "PUT", "/v1/apps/blog/source", map[string]any{"method": "sftp", "host": "web01.lan", "username": "reader",
		"path": "/var/www/html"}, &v)

	// A read by hand: one at a time, a summary checked against its schema.
	boss.must(202, "POST", "/v1/apps/blog/source/reads", nil, nil)
	boss.must(409, "POST", "/v1/apps/blog/source/reads", nil, &apiErr)
	if apiErr.Error != "read_in_progress" {
		t.Fatalf("second read: %s", apiErr.Error)
	}
	var job struct {
		ID       int64
		Method   string
		Host     string
		Path     string
		Password *string
	}
	if readerCall(t, e, tok.Token, "/v1/source/claim", nil, &job) != 200 || job.Method != "sftp" || job.Host != "web01.lan" ||
		job.Path != "/var/www/html" || job.Password != nil {
		t.Fatalf("job: %+v", job)
	}
	if readerCall(t, e, tok.Token, "/v1/source/reads/"+itoa(job.ID)+"/result",
		map[string]any{"summary": map[string]any{"schema": "source-summary/1.0", "code": "<?php"}}, nil) != 422 {
		t.Fatal("a summary off its schema")
	}
	boss.must(200, "GET", "/v1/apps/blog/source", nil, &v)
	if v.LastRead == nil || v.LastRead.Status != "failed" {
		t.Fatalf("refused summary: %+v", v.LastRead)
	}

	// A long error in another alphabet, with a NUL, is stored cut between characters.
	boss.must(202, "POST", "/v1/apps/blog/source/reads", nil, nil)
	readerCall(t, e, tok.Token, "/v1/source/claim", nil, &job)
	if c := readerCall(t, e, tok.Token, "/v1/source/reads/"+itoa(job.ID)+"/result",
		map[string]any{"error": strings.Repeat("ошибка\x00", 200)}, nil); c != 204 {
		t.Fatalf("long error: %d", c)
	}

	// A read no reader takes does not hold the site's read slot for good.
	boss.must(202, "POST", "/v1/apps/blog/source/reads", nil, nil)
	if _, err := e.st.DB.Exec(`UPDATE source_reads SET created_at = now() - interval '20 minutes' WHERE status = 'queued'`); err != nil {
		t.Fatal(err)
	}
	boss.must(200, "GET", "/v1/apps/blog/source", nil, &v)
	if v.LastRead == nil || v.LastRead.Status != "failed" {
		t.Fatalf("a read nobody took: %+v", v.LastRead)
	}
	// A new token ends the reads of the old one.
	boss.must(202, "POST", "/v1/apps/blog/source/reads", nil, nil)
	var renewed struct{ Token string }
	boss.must(201, "POST", "/v1/apps/blog/source/token", nil, &renewed)
	boss.must(200, "GET", "/v1/apps/blog/source", nil, &v)
	if v.LastRead == nil || v.LastRead.Status != "failed" {
		t.Fatalf("a read of the old token: %+v", v.LastRead)
	}
	tok.Token = renewed.Token
	if readerCall(t, e, tok.Token, "/v1/source/hello", map[string]any{"version": "0.2.31", "public_key": key}, nil) != 204 {
		t.Fatal("hello with the renewed token")
	}

	// Another site's reader cannot touch this site's reads.
	var other struct{ Token string }
	boss.must(201, "POST", "/v1/apps/shop/source/token", nil, &other)
	summary, err := os.ReadFile("../../../schema/source/examples/wordpress.json")
	if err != nil {
		t.Fatal(err)
	}

	// A discovery that asks for the source waits for the read, then gets it.
	var d struct {
		ID         int64
		WithSource bool `json:"with_source"`
	}
	boss.must(201, "POST", "/v1/apps/blog/discoveries", start, &d)
	if !d.WithSource {
		t.Fatal("with_source")
	}
	if internalPost(t, e, "/v1/internal/discoveries/claim", nil, nil) != 204 {
		t.Fatal("the discovery did not wait for the read")
	}
	readerCall(t, e, tok.Token, "/v1/source/claim", nil, &job)
	if readerCall(t, e, other.Token, "/v1/source/reads/"+itoa(job.ID)+"/result", map[string]any{"summary": json.RawMessage(summary)}, nil) != 404 {
		t.Fatal("another site's reader answered this read")
	}
	if readerCall(t, e, tok.Token, "/v1/source/reads/"+itoa(job.ID)+"/result", map[string]any{"summary": json.RawMessage(summary)}, nil) != 204 {
		t.Fatal("result")
	}
	var dj struct {
		ID           int64
		Source       map[string]any
		SourceStatus *string `json:"source_status"`
	}
	if internalPost(t, e, "/v1/internal/discoveries/claim", nil, &dj) != 200 || dj.ID != d.ID || dj.SourceStatus == nil ||
		*dj.SourceStatus != "used" || dj.Source["stack"] != "wordpress" {
		t.Fatalf("claim: %+v", dj)
	}
	boss.must(200, "GET", "/v1/apps/blog/source", nil, &v)
	if v.LastRead.Status != "done" || v.LastRead.Counts == nil || v.LastRead.Counts.Routes == 0 {
		t.Fatalf("counts: %+v", v.LastRead)
	}

	// Forgetting the reader stops its token.
	viewer.must(403, "DELETE", "/v1/apps/blog/source", nil, nil)
	boss.must(204, "DELETE", "/v1/apps/blog/source", nil, nil)
	if readerCall(t, e, tok.Token, "/v1/source/hello", map[string]any{"version": "0.2.29"}, nil) != 401 {
		t.Fatal("token after delete")
	}
}
