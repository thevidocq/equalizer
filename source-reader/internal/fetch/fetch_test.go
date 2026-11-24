package fetch

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
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

// The listing decides what is copied: secrets, uploads and other files
// never cross the network.
func TestSMBWanted(t *testing.T) {
	listing := strings.Join([]string{
		"  .                                   D        0  Mon Nov 24 10:00:00 2026",
		"  index.php                           A      418  Mon Nov 24 10:00:00 2026",
		"  wp-config.php                       A     3000  Mon Nov 24 10:00:00 2026",
		"  my file.php                         A      100  Mon Nov 24 10:00:00 2026",
		"  odd;name.php                        A      100  Mon Nov 24 10:00:00 2026",
		"  logo.png                            A     9000  Mon Nov 24 10:00:00 2026",
		"  wp-content                          D        0  Mon Nov 24 10:00:00 2026",
		"",
		`\Blog\wp-content`,
		"  plugin.php                          AR     200  Mon Nov 24 10:00:00 2026",
		"  uploads                             D        0  Mon Nov 24 10:00:00 2026",
		"",
		`\Blog\wp-content\uploads`,
		"  shell.php                           A      200  Mon Nov 24 10:00:00 2026",
		"",
		`\Other`,
		"  x.php                               A      200  Mon Nov 24 10:00:00 2026",
		"\t\t12345 blocks of size 4096. 100 blocks available",
	}, "\r\n")
	got := strings.Join(SMBWanted(listing, "/blog"), ",")
	if got != "index.php,my file.php,wp-content/plugin.php" {
		t.Fatalf("got %s", got)
	}
}

// The configuration with credentials of every supported application, by
// name, and a password written in a file with another name.
func TestSecrets(t *testing.T) {
	for _, rel := range []string{
		"wp-config.php", "configuration.php", "sites/default/settings.php", "sites/default/settings.local.php",
		"app/etc/env.php", "app/config/parameters.php", "config/settings.inc.php", "config.php", "admin/config.php",
		"typo3conf/LocalConfiguration.php", "typo3conf/AdditionalConfiguration.php", "config/system/settings.php",
		"config/system/additional.php", "LocalSettings.php", "include/db.php", "inc/dbconnect.php", "lib/database.php",
		"Config.PHP", "secrets.php", ".env.production", "auth.json",
	} {
		if Wanted(rel, 10) {
			t.Errorf("%s is read", rel)
		}
	}
	for _, src := range []string{
		`<?php define('DB_PASSWORD', 'hunter22');`,
		`<?php $db_pass = "hunter22";`,
		`<?php return ['password' => 'hunter22'];`,
		`<?php $c = mysqli_connect("localhost", "root", "hunter22");`,
		`<?php $pdo = new PDO('mysql:host=x', 'root', 'hunter22');`,
	} {
		if !SecretContent(src) {
			t.Errorf("not caught: %s", src)
		}
	}
	for _, src := range []string{
		`<?php $password = $_POST['password'];`, `<?php $pass = '';`, `<?php $c = mysqli_connect($h, $u, $p);`,
	} {
		if SecretContent(src) {
			t.Errorf("code without a password dropped: %s", src)
		}
	}
}

func TestSMBWantedStaysInside(t *testing.T) {
	listing := "\\Blog\\..\\..\\etc\r\n  passwd.php                          A      200  Mon Nov 24 10:00:00 2026\r\n" +
		"  ..                                  A      200  Mon Nov 24 10:00:00 2026\r\n"
	if got := SMBWanted(listing, "/blog"); len(got) != 0 {
		t.Fatalf("%v", got)
	}
}

func TestGuarded(t *testing.T) {
	dir := t.TempDir()
	err := guarded(context.Background(), dir, func(ctx context.Context) error {
		for i := 0; i <= MaxFiles; i++ {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			_ = os.WriteFile(filepath.Join(dir, strconv.Itoa(i)+".php"), nil, 0o600)
		}
		<-ctx.Done()
		return ctx.Err()
	})
	if err == nil || !strings.Contains(err.Error(), "read stopped") {
		t.Fatalf("%v", err)
	}
}
