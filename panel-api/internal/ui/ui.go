// Package ui serves the panel-ui build: static files, and index.html for
// every other path so client-side routes survive a reload.
package ui

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Handler serves dir as a single-page application.
func Handler(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	index := filepath.Join(dir, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		clean := path.Clean("/" + r.URL.Path)
		if fi, err := os.Stat(filepath.Join(dir, filepath.FromSlash(clean))); err == nil && !fi.IsDir() {
			if strings.HasPrefix(clean, "/assets/") {
				// Hashed file names: cache for a year.
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			files.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, index)
	})
}
