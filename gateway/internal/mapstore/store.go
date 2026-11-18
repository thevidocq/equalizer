// Package mapstore loads, verifies and keeps the signed map active.
package mapstore

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/thevidocq/equalizer/gateway/internal/site"
)

// Store holds the active site and swaps it atomically.
// A request uses a single Site from start to finish.
type Store struct {
	source Source
	pub    ed25519.PublicKey
	keyID  string
	opts   site.Options
	active atomic.Pointer[site.Site]
}

// NewStore creates a store that reads the signed document from a file.
func NewStore(path string, pub ed25519.PublicKey, keyID string, opts site.Options) *Store {
	return NewStoreFrom(&FileSource{Path: path}, pub, keyID, opts)
}

// NewStoreFrom creates a store over any source (file or panel).
func NewStoreFrom(src Source, pub ed25519.PublicKey, keyID string, opts site.Options) *Store {
	return &Store{source: src, pub: pub, keyID: keyID, opts: opts}
}

// Source returns where the map comes from.
func (s *Store) Source() Source { return s.source }

// Current returns the active site, or nil if no map has been loaded.
func (s *Store) Current() *site.Site {
	return s.active.Load()
}

// Reload fetches, verifies, compiles and activates the map. On error the
// previous site stays in use (fail closed only when there is none). It
// returns ErrUnchanged when the source has nothing new.
func (s *Store) Reload() (*site.Site, error) {
	return s.ReloadContext(context.Background())
}

// ReloadContext is Reload with a context.
func (s *Store) ReloadContext(ctx context.Context) (*site.Site, error) {
	doc, err := s.source.Fetch(ctx)
	if err != nil {
		return nil, err
	}
	m, _, err := Verify(doc, s.pub, s.keyID)
	if err != nil {
		s.forget()
		return nil, err
	}
	st, err := site.BuildWith(m, s.opts)
	if err != nil {
		s.forget()
		return nil, fmt.Errorf("unusable map: %w", err)
	}
	s.active.Store(st)
	return st, nil
}

// forget makes the source deliver the document again: a rejected map is
// retried (and logged) at the next check instead of being skipped silently.
func (s *Store) forget() {
	if f, ok := s.source.(interface{ Forget() }); ok {
		f.Forget()
	}
}

// IsUnchanged reports the "nothing new" outcome of Reload.
func IsUnchanged(err error) bool { return errors.Is(err, ErrUnchanged) }
