// Package fetch brings a site's code files where the reader can read them:
// in place for a mounted folder, into a work folder over sftp (rsync over
// ssh) or smb (smbclient). Only code files, never secrets, never uploads.
package fetch

import (
	"path"
	"regexp"
	"strings"
)

// Limits of one read. The copy's are wider than the study's: an
// application's core (Moodle's, Magento's) has tens of thousands of files
// the reader only takes the version from, and it must reach the site's own
// code past them.
const (
	MaxFileSize  = 1 << 20   // a larger file is not code a person wrote
	MaxFiles     = 20000     // files studied: past this the summary says it was cut
	MaxBytes     = 200 << 20 // studied, all files together
	MaxCopyFiles = 150000    // files copied from the server
	MaxCopyBytes = 1 << 30   // copied, all files together
)

// codeExt and codeNames are the files the reader reads: code, and the few
// files that say what a component is or where an application's entry
// points are (style.css headers, composer files, routing files, manifests).
var codeExt = map[string]bool{".php": true, ".phtml": true, ".inc": true, ".module": true}
var codeNames = []string{"style.css", "composer.lock", "composer.json", "*.routing.yml", "routing.yml", "routes.xml",
	"webapi.xml", "extension.json", "skin.json", "*.info.yml", "*.info", "install.json"}

// SecretFiles are the files with credentials of each application Equalizer
// supports, from their own documentation: never read, wherever they are.
var SecretFiles = map[string][]string{
	"wordpress":  {"wp-config.php", "wp-config-*.php", "wp-config.*.php"},
	"joomla":     {"configuration.php", "configuration.php-*"},
	"drupal":     {"settings.php", "settings.*.php", "settings-*.php"},
	"magento":    {"env.php", "local.xml"},
	"prestashop": {"parameters.php", "parameters.yml", "settings.inc.php"},
	"opencart":   {"config.php"},
	"typo3":      {"LocalConfiguration.php", "AdditionalConfiguration.php", "additional.php", "settings.php"},
	"moodle":     {"config.php"},
	"mediawiki":  {"LocalSettings.php", "LocalSettings.*.php"},
	"phpbb":      {"config.php", "config_*.php"},
	// a custom site: the names its database settings usually have
	"generic": {"config.php", "config.inc.php", "config.*.php", "config_*.php", "config-*.php", "*.config.php", "*.conf.php",
		"configuration*.php", "conf.php", "conf.inc.php", "db.php", "db.inc", "db.inc.php", "db_*.php", "db-*.php", "dbconfig*.php",
		"dbconn*.php", "dbconnect*.php", "database*.php", "connect.php", "connection*.php", "conn.php", "conn_*.php", "config.inc",
		"settings*.php", "secrets*.php", "credentials*.php", "credential*.php", "passwords*.php", "local_settings.php",
		"localconfig*.php", "parameters*.php"},
	// keys, certificates and environment files, on any site
	"keys": {".env", ".env.*", "*.env", "*.pem", "*.key", "*.p12", "*.pfx", "*.jks", "*.kdbx", "id_rsa*", "id_ecdsa*", "id_ed25519*",
		".htpasswd", ".netrc", ".pgpass", ".my.cnf", "auth.json"},
}

var secretNames = func() []string {
	var out []string
	for _, names := range SecretFiles {
		out = append(out, names...)
	}
	return out
}()

// skipDirs are folders never entered: uploads, caches, version control,
// dependencies installed by a tool (their versions come from composer.lock).
var skipDirs = []string{".git", ".svn", ".hg", "node_modules", "vendor", "bower_components", "uploads", "cache", "caches", "tmp",
	"temp", "backup", "backups", ".well-known"}

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
	if codeExt[strings.ToLower(path.Ext(base))] {
		return true
	}
	for _, n := range codeNames {
		if ok, _ := path.Match(n, base); ok {
			return true
		}
	}
	return false
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

// Secret reports whether a file's name is one never read; names are
// compared without case (Windows servers, smb).
func Secret(base string) bool {
	low := strings.ToLower(base)
	for _, p := range secretNames {
		if ok, _ := path.Match(strings.ToLower(p), low); ok {
			return true
		}
	}
	return false
}

// A file whose name the lists above do not know but that holds a password
// written in the code (a custom site's settings under another name): the
// reader drops it at once, before reading anything in it.
var secretContent = []*regexp.Regexp{
	regexp.MustCompile(`(?i)define\(\s*['"][A-Z0-9_]*(PASS|PWD|SECRET|KEY|SALT|TOKEN)[A-Z0-9_]*['"]\s*,\s*['"][^'"]{3,}['"]`),
	regexp.MustCompile(`(?i)\$[A-Za-z0-9_]*(pass|pwd|secret|apikey|api_key|token)[A-Za-z0-9_]*\s*=\s*['"][^'"]{3,}['"]`),
	regexp.MustCompile(`(?i)['"][A-Za-z0-9_]*(pass|pwd|secret|apikey|api_key)[A-Za-z0-9_]*['"]\s*=>\s*['"][^'"]{3,}['"]`),
	regexp.MustCompile(`(?i)(mysqli_connect|mysql_connect|mysql_pconnect|pg_connect|sqlsrv_connect|oci_connect|new\s+mysqli|new\s+PDO)\s*\([^)]*['"][^'"]+['"]\s*,\s*['"][^'"]+['"]\s*,\s*['"][^'"]{3,}['"]`),
}

// SecretContent reports whether a file's text holds a password written in
// the code: such a file is dropped, never read further.
func SecretContent(src string) bool {
	for _, re := range secretContent {
		if re.MatchString(src) {
			return true
		}
	}
	return false
}
