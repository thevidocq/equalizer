// Package fetch brings a site's code files where the reader can read them:
// in place for a mounted folder, into a work folder over sftp (rsync over
// ssh) or smb (smbclient). Only code files, never secrets, never uploads.
package fetch

import (
	"path"
	"strings"
)

// Limits of one read.
const (
	MaxFileSize = 1 << 20   // a larger file is not code a person wrote
	MaxFiles    = 20000     // past this the summary says it was cut
	MaxBytes    = 200 << 20 // all files together
)

// codeNames are the files the reader reads: code, and the few files that
// say what a component is (style.css headers, composer.lock).
var codeExt = map[string]bool{".php": true, ".phtml": true, ".inc": true}
var codeNames = map[string]bool{"style.css": true, "composer.lock": true}

// secret are files never read, whatever their extension: credentials,
// keys and configuration with passwords.
var secretNames = []string{"wp-config.php", "wp-config-*.php", "wp-config.*.php", ".env", ".env.*", "*.pem", "*.key", "*.p12", "*.pfx",
	"id_rsa*", "id_ed25519*", ".htpasswd", "config.inc.php", "settings.php", "local.xml", "env.php", "parameters.yml", "configuration.php", "LocalConfiguration.php"}

// skipDirs are folders never entered: uploads, caches, version control,
// dependencies installed by a tool (their versions come from the lock file).
var skipDirs = []string{".git", ".svn", ".hg", "node_modules", "uploads", "cache", "caches", "tmp", "temp", "backup", "backups", ".well-known"}

// Wanted reports whether a file (path relative to the site's folder, with
// "/") is one the reader reads.
func Wanted(rel string, size int64) bool {
	if size > MaxFileSize || rel == "" {
		return false
	}
	for _, d := range strings.Split(path.Dir(rel), "/") {
		if SkipDir(d) {
			return false
		}
	}
	base := path.Base(rel)
	if Secret(base) {
		return false
	}
	return codeExt[strings.ToLower(path.Ext(base))] || codeNames[base]
}

// SkipDir reports whether a folder's name is one never entered.
func SkipDir(name string) bool {
	for _, d := range skipDirs {
		if strings.EqualFold(name, d) {
			return true
		}
	}
	return false
}

// Secret reports whether a file's name is one never read.
func Secret(base string) bool {
	for _, p := range secretNames {
		if ok, _ := path.Match(p, base); ok {
			return true
		}
	}
	return false
}
