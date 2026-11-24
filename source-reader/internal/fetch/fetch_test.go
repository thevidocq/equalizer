package fetch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWanted(t *testing.T) {
	for rel, want := range map[string]bool{
		"index.php": true, "wp-content/plugins/a/a.php": true, "wp-content/themes/t/style.css": true, "composer.lock": true,
		"wp-config.php": false, "wp-config-sample.php": false, ".env": false, ".env.local": false, "certs/site.key": false,
		"wp-content/uploads/x.php": false, ".git/hooks/a.php": false, "node_modules/x/y.php": false, "readme.txt": false,
		"app/settings.php": false, "img/logo.png": false,
	} {
		if Wanted(rel, 100) != want {
			t.Errorf("%s: want %v", rel, want)
		}
	}
	if Wanted("big.php", MaxFileSize+1) {
		t.Error("a huge file")
	}
}

// rsync filters: secrets and skipped folders before any include, and
// everything else excluded at the end.
func TestRsyncArgs(t *testing.T) {
	args := RsyncArgs(Spec{Method: "sftp", Host: "web.example.com", Username: "reader", Path: "/var/www/html"},
		Env{Key: "/state/id_ed25519", State: "/state"}, "/work/read-1")
	joined := strings.Join(args, " ")
	first := func(s string) int { return strings.Index(joined, s) }
	if first("--exclude=wp-config.php") > first("--include=*.php") || first("--exclude=uploads/") > first("--include=*/") {
		t.Fatalf("secrets must come first: %s", joined)
	}
	if args[len(args)-3] != "--exclude=*" || args[len(args)-2] != "reader@web.example.com:/var/www/html/" ||
		!strings.Contains(joined, "StrictHostKeyChecking=accept-new") || !strings.Contains(joined, "-p 22 ") {
		t.Fatalf("%s", joined)
	}
}

func TestFetchRefusesInjection(t *testing.T) {
	for _, s := range []Spec{
		{Method: "sftp", Host: "web.example.com; rm -rf /", Path: "/x"},
		{Method: "sftp", Host: "web.example.com", Username: "-oProxyCommand=x", Path: "/x"},
		{Method: "smb", Host: "fs", Share: "a/b", Path: "/x"},
		{Method: "sftp", Host: "web", Path: "/x$(id)"},
		{Method: "sftp", Host: "web", Path: "/var/www; touch /tmp/x"},
		{Method: "sftp", Host: "web", Path: "/var/www|id"},
		{Method: "sftp", Host: "web", Path: "/var/../etc"},
		{Method: "sftp", Host: "-oProxyCommand=id", Path: "/x"},
		{Method: "smb", Host: "fs", Share: "www", Path: "/x; !id"},
		{Method: "smb", Host: "fs", Share: "www", Path: "/x", Password: "pw\ndomain = EVIL"},
	} {
		if _, clean, err := Fetch(context.Background(), s, Env{Work: t.TempDir()}); err == nil {
			clean()
			t.Errorf("accepted %+v", s)
		}
	}
}

func TestLocalAndPrune(t *testing.T) {
	site := t.TempDir()
	if err := os.MkdirAll(filepath.Join(site, "www", "uploads"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"www/index.php", "www/wp-config.php", "www/uploads/a.php", "www/logo.png"} {
		_ = os.WriteFile(filepath.Join(site, f), []byte("<?php"), 0o644)
	}
	dir, clean, err := Fetch(context.Background(), Spec{Method: "local", Path: "/www/../www"}, Env{Site: site})
	defer clean()
	if err != nil || dir != filepath.Join(site, "www") {
		t.Fatalf("%s %v", dir, err)
	}
	if _, _, err := Fetch(context.Background(), Spec{Method: "local", Path: "/missing"}, Env{Site: site}); err == nil {
		t.Error("a missing folder")
	}
	if err := Prune(dir); err != nil {
		t.Fatal(err)
	}
	var left []string
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, _ error) error {
		if !info.IsDir() {
			left = append(left, filepath.Base(p))
		}
		return nil
	})
	if strings.Join(left, ",") != "index.php" {
		t.Fatalf("left: %v", left)
	}
}
