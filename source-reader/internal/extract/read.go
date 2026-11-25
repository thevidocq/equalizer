package extract

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/thevidocq/equalizer/source-reader/internal/fetch"
)

// Read reads a site's folder and returns its summary.
func Read(root string, now time.Time) (*Summary, error) {
	s := &Summary{Schema: "source-summary/1.0", ReadAt: now.UTC(), Stack: "unknown",
		Routes: []Route{}, Inputs: []Input{}, Risks: []Risk{}, Components: []Component{}}
	if s.app = detect(root); s.app != nil {
		s.Stack = s.app.name
	}
	var total int64
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable: skipped
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel != "." && fetch.SkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 { // never followed: it may lead out of the site
			s.Files.Skipped++
			return nil
		}
		info, err := d.Info()
		// a regular file only: a FIFO named x.php would block the read for
		// good, a device would never end
		if err != nil || !info.Mode().IsRegular() || !fetch.Wanted(rel, info.Size()) {
			s.Files.Skipped++
			return nil
		}
		if s.known(rel) {
			s.Files.Known++
			return nil
		}
		if s.Files.Read >= fetch.MaxFiles || total+info.Size() > fetch.MaxBytes {
			s.Files.Truncated = true
			return filepath.SkipAll
		}
		data, err := readAtMost(p, fetch.MaxFileSize)
		if err != nil {
			s.Files.Skipped++
			return nil
		}
		if fetch.SecretContent(string(data)) { // a password in the code: dropped unread
			s.Files.Skipped++
			return nil
		}
		total += int64(len(data))
		s.Files.Read++
		s.file0 = rel
		s.file(rel, string(data))
		return nil
	})
	if err != nil {
		return nil, err
	}
	if s.app != nil && s.app.finish != nil {
		s.app.finish(s, s.app)
	}
	if s.Stack == "unknown" && s.sawPHP {
		s.Stack = "php"
	}
	s.fit()
	s.sort()
	return s, nil
}

// file reads one file by its kind: what the application declares in it,
// then, for PHP outside the application's core, its entry point, inputs and
// risky calls.
func (s *Summary) file(rel, src string) {
	a := s.app
	in, ok := a.within(rel)
	s.owner = ""
	if ok && a.owner != nil {
		if kind, name := a.owner(a, in); name != "" {
			s.owner = kind + " " + name
		}
	}
	if path.Base(rel) == "composer.lock" {
		s.composer(rel, src)
		return
	}
	core := ok && a.core != nil && a.core(a, in)
	if ok && a.meta != nil {
		a.meta(s, a, in, src)
	}
	if ok && !core && !library(rel) && a.file != nil {
		a.file(s, a, in, src)
	}
	if !isPHP(rel) {
		return
	}
	s.sawPHP = true
	if core || library(rel) {
		return // known code: only its version
	}
	route := ""
	switch {
	case strings.ToLower(path.Ext(rel)) != ".php":
	case ok && a.direct != nil:
		if a.direct(a, in, src) {
			route = a.path(in)
		}
	case ok:
		if acts(src) {
			route = a.path(in)
		}
	case a != nil && a.web: // outside the web server's root: no browser opens it
	case acts(src):
		route = "/" + rel
	}
	if route != "" {
		s.declared(Route{Path: route, Methods: []string{"GET", "POST"}, Origin: "php_file", Public: true, Where: s.at(1)})
	}
	s.phpFile(rel, src, route)
}

// at is a line of the file being read.
func (s *Summary) at(line int) Where {
	return Where{File: s.file0, Line: line, Owner: s.owner}
}

// composer reads the libraries of a composer.lock.
func (s *Summary) composer(rel, src string) {
	var lock struct {
		Packages []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"packages"`
	}
	if json.Unmarshal([]byte(src), &lock) != nil {
		return
	}
	for _, p := range lock.Packages {
		if p.Name == "" || len(p.Name) > 120 {
			continue
		}
		v := strings.TrimPrefix(p.Version, "v")
		if s.app != nil && slices.Contains(s.app.corePackages, p.Name) && (rel == "composer.lock" || rel == s.app.base+"composer.lock") {
			s.version(s.app.name, v)
		}
		c := Component{Name: "library:" + p.Name, Kind: "library"}
		if v != "" && len(v) <= 40 {
			c.Version = &v
		}
		s.addComponent(c)
	}
}

// sort makes two reads of the same files give the same summary.
func (s *Summary) sort() {
	key := func(r Route) string {
		k := r.Path + "\x00" + r.Action + "\x00" + strings.Join(r.Methods, ",")
		for _, q := range sortedKeys(r.Query) {
			k += "\x00" + q + "=" + r.Query[q]
		}
		return k + "\x00" + r.Where.File
	}
	sort.SliceStable(s.Routes, func(i, j int) bool { return key(s.Routes[i]) < key(s.Routes[j]) })
	sort.SliceStable(s.Inputs, func(i, j int) bool {
		a, b := s.Inputs[i], s.Inputs[j]
		return a.Name+a.Source+a.Where.File < b.Name+b.Source+b.Where.File ||
			a.Name+a.Source+a.Where.File == b.Name+b.Source+b.Where.File && a.Where.Line < b.Where.Line
	})
	sort.SliceStable(s.Risks, func(i, j int) bool {
		a, b := s.Risks[i], s.Risks[j]
		return a.Where.File < b.Where.File || a.Where.File == b.Where.File && a.Where.Line < b.Where.Line
	})
	sort.SliceStable(s.Components, func(i, j int) bool { return s.Components[i].Name < s.Components[j].Name })
}

// fit keeps what the schema can hold: an entry whose path, name or place
// is longer than its field is left out, a long version is dropped. One odd
// file never fails the whole read.
func (s *Summary) fit() {
	short := func(v string, n int) bool { return len(v) <= n }
	routes := s.Routes[:0]
	for _, r := range s.Routes {
		ok := short(r.Path, 300) && short(r.Action, 100) && short(r.Where.File, 300) && short(r.Where.Owner, 140) && len(r.Query) <= 5
		for k, v := range r.Query {
			ok = ok && short(k, 100) && short(v, 100)
		}
		if ok {
			routes = append(routes, r)
		}
	}
	s.Routes = routes
	inputs := s.Inputs[:0]
	for _, in := range s.Inputs {
		if short(in.Name, 100) && short(in.Route, 300) && short(in.Where.File, 300) && short(in.Where.Owner, 140) {
			inputs = append(inputs, in)
		}
	}
	s.Inputs = inputs
	risks := s.Risks[:0]
	for _, r := range s.Risks {
		if short(r.Input, 100) && short(r.Route, 300) && short(r.Where.File, 300) && short(r.Where.Owner, 140) {
			risks = append(risks, r)
		}
	}
	s.Risks = risks
	comps := s.Components[:0]
	for _, c := range s.Components {
		if !short(c.Name, 120) {
			continue
		}
		if c.Version != nil && !short(*c.Version, 40) {
			c.Version = nil
		}
		comps = append(comps, c)
	}
	s.Components = comps
}

// readAtMost reads a file, or fails when it holds more than n bytes (it
// grew since it was listed).
func readAtMost(p string, n int64) ([]byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, n+1))
	if err == nil && int64(len(b)) > n {
		err = errors.New("larger than the limit")
	}
	return b, err
}
