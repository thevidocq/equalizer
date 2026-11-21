package pathkey

import "testing"

func TestOf(t *testing.T) {
	for in, want := range map[string]string{
		"/":                     "/",
		"/admin/":               "/admin/",
		"//admin/x":             "/admin/x",
		"/wp-admin//ajax.php":   "/wp-admin/ajax.php",
		"/admin;jsessionid=1/x": "/admin/x",
		"/manager;a/html":       "/manager/html",
		"/admin./x":             "/admin/x",
		"/admin. . /x":          "/admin/x",
		"/a/":                   "/a/",
		"/a//":                  "/a/",
		"/..;/admin":            "/../admin",
		"/a/../b":               "/a/../b",
		"/file.tar.gz":          "/file.tar.gz",
		"/x;":                   "/x",
	} {
		if got := Of(in); got != want {
			t.Errorf("Of(%q) = %q, want %q", in, got, want)
		}
	}
}

func FuzzOf(f *testing.F) {
	for _, s := range []string{"/", "//a//b/", "/a;b/c", "/a. /", "/..;/x", "/%2e/", "/a/b/c.php/x"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, p string) {
		k := Of(p)
		if Of(k) != k {
			t.Fatalf("not stable: %q -> %q -> %q", p, k, Of(k))
		}
		if len(p) > 0 && p[0] == '/' && (len(k) == 0 || k[0] != '/') {
			t.Fatalf("lost the leading slash: %q -> %q", p, k)
		}
	})
}
