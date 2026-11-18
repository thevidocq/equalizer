// Package mapstore loads, verifies and keeps the signed map active.
package mapstore

import (
	"crypto/ed25519"
	"fmt"
	"os"
	"sync/atomic"

	"github.com/thevidocq/equalizer/gateway/internal/site"
)

// Store holds the active site and swaps it atomically.
// A request uses a single Site from start to finish.
type Store struct {
	path   string
	pub    ed25519.PublicKey
	keyID  string
	opts   site.Options
	active atomic.Pointer[site.Site]
}

// NewStore creates a store that reads the signed document from path.
func NewStore(path string, pub ed25519.PublicKey, keyID string, opts site.Options) *Store {
	return &Store{path: path, pub: pub, keyID: keyID, opts: opts}
}

// Current returns the active site, or nil if no map has been loaded.
func (s *Store) Current() *site.Site {
	return s.active.Load()
}

// Reload reads, verifies, compiles and activates the map. On error the
// previous site stays in use (fail closed only when there is none).
func (s *Store) Reload() (*site.Site, error) {
	doc, err := os.ReadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("reading map: %w", err)
	}
	m, _, err := Verify(doc, s.pub, s.keyID)
	if err != nil {
		return nil, err
	}
	st, err := site.BuildWith(m, s.opts)
	if err != nil {
		return nil, fmt.Errorf("unusable map: %w", err)
	}
	s.active.Store(st)
	return st, nil
}
