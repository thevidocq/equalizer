package extract

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/thevidocq/equalizer/source-reader/internal/fetch"
)

// Each application Equalizer supports declares its entry points its own
// way (REST routes, routing files, components, controllers) and ships a
// core that is known code. A kind says how to find the application, which
// files are its core, who owns a file, what a file declares, and which PHP
// files a browser opens directly.

type kind struct {
	name string
	// markers: sets of files under the application's folder that only it
	// has; one whole set is enough (a layout, a version)
	markers [][]string
	// probe finds the application when no marker set does (TYPO3 installed
	// with Composer: its core is in vendor/, which the reader never copies)
	probe func(root, base string) bool
	// versions: files of the core read for its version; the core's other
	// files are known code, never read
	versions []string
	// corePackages: composer packages whose version is the core's
	corePackages []string
	// load reads a few files of the core once found (lists of the core's
	// own extensions); its result is app.data
	load func(root string) any
	// core: shipped with the application (rel is under its folder)
	core func(a *app, rel string) bool
	// owner: the extension a file belongs to ("plugin", "akismet"), or ""
	owner func(a *app, rel string) (string, string)
	// meta reads versions, from any file of the application, core included
	meta func(s *Summary, a *app, rel, src string)
	// file reads what a file outside the core declares: routes, inputs
	file func(s *Summary, a *app, rel, src string)
	// direct: a PHP file outside the core that a browser can open and that
	// runs on its own (no guard against direct access); nil: acts() says
	direct func(a *app, rel, src string) bool
	// finish combines what several files said (Magento's controllers)
	finish func(s *Summary, a *app)
}

// app is the application found in the site, at base ("" or "blog/").
type app struct {
	*kind
	base string
	web  bool // base is the web server's root: its path is not in the URL
	data any
	// collected by file, combined by finish
	front map[string]string   // Magento: module folder → frontName
	admin map[string]string   // Magento: module folder → admin frontName
	ctrl  map[string][]string // Magento: module folder → controller paths
}

// kinds, in the order they are tried: the more specific first.
var kinds = []*kind{wordpressKind, drupalKind, drupal7Kind, joomlaKind, magentoKind, prestashopKind, opencartKind,
	typo3Kind, mediawikiKind, phpbbKind, moodleKind}

// detect finds the application at the site's root or in one of its
// folders (WordPress in /blog, Drupal in /web, Moodle 5 in /public).
func detect(root string) *app {
	try := func(base string) *app {
		for _, k := range kinds {
			if k.found(root, base) || k.probe != nil && k.probe(root, base) {
				a := &app{kind: k, base: base, web: webRoots[strings.TrimSuffix(base, "/")],
					front: map[string]string{}, admin: map[string]string{}, ctrl: map[string][]string{}}
				if k.load != nil {
					a.data = k.load(filepath.Join(root, filepath.FromSlash(base)))
				}
				return a
			}
		}
		return nil
	}
	if a := try(""); a != nil {
		return a
	}
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if e.IsDir() && !fetch.SkipDir(e.Name()) && !strings.HasPrefix(e.Name(), ".") {
			if a := try(e.Name() + "/"); a != nil {
				return a
			}
		}
	}
	return nil
}

func (k *kind) found(root, base string) bool {
	for _, set := range k.markers {
		ok := true
		for _, m := range set {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(base+m))); err != nil {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// within is rel under the application's folder.
func (a *app) within(rel string) (string, bool) {
	if a == nil || !strings.HasPrefix(rel, a.base) {
		return "", false
	}
	return strings.TrimPrefix(rel, a.base), true
}

// webRoots are the names of a folder the web server serves as the site's
// root (Drupal's web/, Moodle 5's public/), not a folder under it (blog/).
var webRoots = map[string]bool{"web": true, "public": true, "public_html": true, "docroot": true, "htdocs": true, "html": true,
	"www": true, "upload": true}

// path is the public path of p under the application's folder.
func (a *app) path(p string) string {
	if a.web {
		return "/" + strings.TrimPrefix(p, "/")
	}
	return "/" + a.base + strings.TrimPrefix(p, "/")
}

// known: a file of the application's core, or of a library, that is not
// read: only the core's version files are.
func (s *Summary) known(rel string) bool {
	if path.Base(rel) == "composer.lock" {
		return false
	}
	in, ok := s.app.within(rel)
	if ok && s.app.core != nil && s.app.core(s.app, in) {
		return !slices.Contains(s.app.versions, in)
	}
	return library(rel)
}

// readCore reads a small file of the application's core, "" when missing.
func readCore(root, rel string) string {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil || len(b) > fetch.MaxFileSize {
		return ""
	}
	return string(b)
}

// under tells whether rel is in one of the folders.
func under(rel string, dirs ...string) bool {
	for _, d := range dirs {
		if strings.HasPrefix(rel, d+"/") {
			return true
		}
	}
	return false
}

// segment is the n-th folder of rel ("" when it has fewer).
func segment(rel string, n int) string {
	parts := strings.Split(rel, "/")
	if n >= len(parts)-1 {
		return ""
	}
	return parts[n]
}

func isPHP(rel string) bool {
	switch strings.ToLower(path.Ext(rel)) {
	case ".php", ".phtml", ".inc":
		return true
	}
	return false
}

// declared adds a route the application's files declare, once.
func (s *Summary) declared(r Route) {
	k := r.Path + "|" + r.Origin + "|" + r.Action + "|" + strings.Join(r.Methods, ",")
	for _, q := range sortedKeys(r.Query) {
		k += "|" + q + "=" + r.Query[q]
	}
	if s.routeSeen == nil {
		s.routeSeen = map[string]bool{}
	}
	if s.routeSeen[k] || len(s.Routes) >= maxRoutes {
		return
	}
	s.routeSeen[k] = true
	s.Routes = append(s.Routes, r)
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// version adds the application's core with its version.
func (s *Summary) version(name, v string) {
	v = strings.TrimSpace(v)
	c := Component{Name: name, Kind: "core"}
	if v != "" {
		c.Version = &v
	}
	s.addComponent(c)
}

// extension adds an extension (kind plugin or theme) and its version.
func (s *Summary) extension(kind, word, name, v string) {
	c := Component{Name: word + ":" + name, Kind: kind}
	if v = strings.TrimSpace(v); v != "" {
		c.Version = &v
	}
	s.addComponent(c)
}

// addComponent adds c once; a version found later fills one missing.
func (s *Summary) addComponent(c Component) {
	for i, have := range s.Components {
		if have.Name == c.Name {
			if have.Version == nil {
				s.Components[i].Version = c.Version
			}
			return
		}
	}
	if len(s.Components) < maxComponents {
		s.Components = append(s.Components, c)
	}
}

// guarded: the file refuses to run when opened directly (the guard is at
// its top).
func guarded(re *regexp.Regexp, src string) bool {
	return re.MatchString(src[:min(len(src), 4096)])
}
