package extract

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/thevidocq/equalizer/source-reader/internal/fetch"
)

// Read reads a site's folder and returns its summary.
func Read(root string, now time.Time) (*Summary, error) {
	s := &Summary{Schema: "source-summary/1.0", ReadAt: now.UTC(), Stack: "unknown",
		Routes: []Route{}, Inputs: []Input{}, Risks: []Risk{}, Components: []Component{}}
	if _, err := os.Stat(filepath.Join(root, "wp-includes", "version.php")); err == nil {
		s.Stack = "wordpress"
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
		if err != nil || !fetch.Wanted(rel, info.Size()) {
			s.Files.Skipped++
			return nil
		}
		if s.Files.Read >= fetch.MaxFiles || total+info.Size() > fetch.MaxBytes {
			s.Files.Truncated = true
			return filepath.SkipAll
		}
		data, err := os.ReadFile(p)
		if err != nil {
			s.Files.Skipped++
			return nil
		}
		total += int64(len(data))
		s.Files.Read++
		s.file(rel, string(data))
		return nil
	})
	if err != nil {
		return nil, err
	}
	if s.Stack == "unknown" && s.sawPHP {
		s.Stack = "php"
	}
	s.sort()
	return s, nil
}

// file reads one file by its kind.
func (s *Summary) file(rel, src string) {
	switch filepath.Base(rel) {
	case "composer.lock":
		s.composer(src)
		return
	case "style.css":
		s.component(rel, src)
		return
	}
	s.sawPHP = true
	if s.Stack == "wordpress" {
		s.component(rel, src)
		s.wordpressFile(rel, src)
	}
	route := directRoute(s.Stack, rel, src)
	if route != "" && len(s.Routes) < maxRoutes {
		s.Routes = append(s.Routes, Route{Path: route, Methods: []string{"GET", "POST"}, Origin: "php_file", Public: true,
			Where: Where{File: rel, Line: 1}})
	}
	s.phpFile(rel, src, route)
}

// composer reads the libraries of a composer.lock.
func (s *Summary) composer(src string) {
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
		c := Component{Name: "library:" + p.Name, Kind: "library"}
		if v != "" && len(v) <= 40 {
			c.Version = &v
		}
		s.addComponent(c)
	}
}

// sort makes two reads of the same files give the same summary.
func (s *Summary) sort() {
	sort.SliceStable(s.Routes, func(i, j int) bool {
		a, b := s.Routes[i], s.Routes[j]
		return a.Path+a.Action+a.Where.File < b.Path+b.Action+b.Where.File
	})
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
