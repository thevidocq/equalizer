package rebuild

import (
	"bytes"
	"net/http"
	"path"
	"strings"
)

// ReasonFileRefused is a file in a form that the legacy server could run
// (a script, a program) or a page sent as a file (stored XSS), whatever
// type and name it claims.
const ReasonFileRefused = "file_refused"

// Extensions a web server may run. The last extension counts for all of
// them; inner ones too for those that some servers run wherever they
// appear in the name (Apache's shell.php.png).
var runnable = map[string]bool{
	"php": true, "php3": true, "php4": true, "php5": true, "php7": true, "php8": true, "phtml": true,
	"pht": true, "phar": true, "phps": true, "shtml": true, "shtm": true, "cgi": true, "pl": true,
	"asp": true, "aspx": true, "asa": true, "ascx": true, "ashx": true, "asmx": true, "jsp": true,
	"jspx": true, "py": true, "rb": true, "sh": true, "exe": true, "dll": true, "bat": true,
	"cmd": true, "vbs": true, "ps1": true, "war": true,
}
var runnableInside = map[string]bool{
	"php": true, "php3": true, "php4": true, "php5": true, "php7": true, "php8": true, "phtml": true,
	"pht": true, "phar": true, "shtml": true, "cgi": true, "pl": true, "asp": true, "aspx": true, "jsp": true,
}
var serverFiles = map[string]bool{".htaccess": true, ".htpasswd": true, ".user.ini": true, "web.config": true}

// RunnableName reports whether a file name is one a web server could run
// or obey.
func RunnableName(name string) bool { return runnableName(cleanName(name)) }

func runnableName(name string) bool {
	n := strings.ToLower(strings.TrimRight(path.Base(name), ". ")) // Windows drops trailing dots and spaces
	if serverFiles[n] {
		return true
	}
	parts := strings.Split(n, ".")
	if len(parts) < 2 {
		return false
	}
	if runnable[strings.TrimSpace(parts[len(parts)-1])] {
		return true
	}
	for _, p := range parts[1 : len(parts)-1] {
		if runnableInside[strings.TrimSpace(p)] {
			return true
		}
	}
	return false
}

// programs are the starts of executables and scripts.
var programs = [][]byte{
	[]byte("MZ"), []byte("\x7fELF"), []byte("#!"), []byte("\xca\xfe\xba\xbe"), []byte("\xcf\xfa\xed\xfe"),
	[]byte("\xce\xfa\xed\xfe"), []byte("<%"),
}

var phpOpen = []byte("<?php")

// fileCheck looks at a file while it is copied: its first bytes for its
// real kind, and all of it for PHP code (a picture with PHP inside runs
// if the server is ever made to execute it).
type fileCheck struct {
	declared string
	head     []byte
	tail     []byte
	checked  bool
	bad      bool
}

const sniffLen = 512

func (c *fileCheck) Write(p []byte) (int, error) {
	if !c.checked {
		c.head = append(c.head, p[:min(len(p), sniffLen-len(c.head))]...)
		if len(c.head) == sniffLen {
			c.sniff()
		}
	}
	data := bytes.ToLower(append(c.tail, p...))
	if bytes.Contains(data, phpOpen) {
		c.bad = true
	}
	c.tail = append(c.tail[:0], data[max(0, len(data)-len(phpOpen)+1):]...)
	if c.bad {
		return 0, refuse(ReasonFileRefused)
	}
	return len(p), nil
}

// NewFileCheck checks a file copied through it (Write), declared as
// ctype; Done gives the verdict. A refused file is an *Error with
// ReasonFileRefused.
func NewFileCheck(ctype string) interface {
	Write([]byte) (int, error)
	Done() error
} {
	return &fileCheck{declared: ctype}
}

// Done is done, for NewFileCheck's callers.
func (c *fileCheck) Done() error { return c.done() }

// done finishes the check of a file shorter than the sniffed length.
func (c *fileCheck) done() error {
	if !c.checked {
		c.sniff()
	}
	if c.bad {
		return refuse(ReasonFileRefused)
	}
	return nil
}

func (c *fileCheck) sniff() {
	c.checked = true
	for _, p := range programs {
		if bytes.HasPrefix(c.head, p) {
			c.bad = true
			return
		}
	}
	if len(c.head) == 0 {
		return // an empty file
	}
	got, _, _ := strings.Cut(http.DetectContentType(c.head), ";")
	switch {
	case got == "text/html":
		c.bad = true // a page served from the site's own address
	case strings.HasPrefix(c.declared, "image/") && c.declared != "image/svg+xml":
		c.bad = !strings.HasPrefix(got, "image/")
	case c.declared == "application/pdf":
		c.bad = got != "application/pdf"
	}
}
