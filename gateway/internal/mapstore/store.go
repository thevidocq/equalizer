// Package mapstore loads, verifies and keeps the signed maps active: one per
// protected site, chosen by the request's host.
package mapstore

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/thevidocq/equalizer/gateway/internal/site"
)

// Store holds the active sites and swaps them atomically.
// A request uses a single Site from start to finish.
type Store struct {
	source Source
	pub    ed25519.PublicKey
	keyID  string
	opts   site.Options
	active atomic.Pointer[Sites]
}

// Report is the outcome of a reload.
type Report struct {
	Active []*site.Site // sites served after the reload
	Failed []error      // maps refused; the previous version of their site stays
}

// NewStore creates a store that reads the signed document from a file.
func NewStore(path string, pub ed25519.PublicKey, keyID string, opts site.Options) *Store {
	return NewStoreFrom(&FileSource{Path: path}, pub, keyID, opts)
}

// NewStoreFrom creates a store over any source (file or panel).
func NewStoreFrom(src Source, pub ed25519.PublicKey, keyID string, opts site.Options) *Store {
	s := &Store{source: src, pub: pub, keyID: keyID, opts: opts}
	s.active.Store(&Sites{})
	return s
}

// Source returns where the maps come from.
func (s *Store) Source() Source { return s.source }

// For returns the site served on host, or nil.
func (s *Store) For(host string) *site.Site { return s.active.Load().For(host) }

// Ready reports whether at least one site is loaded.
func (s *Store) Ready() bool { return s.active.Load().Len() > 0 }

// Serves reports whether domain has a published map (for on-demand TLS).
func (s *Store) Serves(domain string) bool {
	return s.active.Load().byHost[normalizeHost(domain)] != nil
}

// All returns the active sites.
func (s *Store) All() []*site.Site { return s.active.Load().All() }

// Current returns the only active site, or nil (single-site installations
// and tests).
func (s *Store) Current() *site.Site {
	all := s.All()
	if len(all) != 1 {
		return nil
	}
	return all[0]
}

// Reload fetches, verifies, compiles and activates the maps.
func (s *Store) Reload() (Report, error) { return s.ReloadContext(context.Background()) }

// ReloadContext is Reload with a context. Each map stands alone: a refused
// map keeps the previous version of its site (fail closed only when there
// is none) and does not stop the others; the error joins the refusals. It
// returns ErrUnchanged when the source has nothing new.
func (s *Store) ReloadContext(ctx context.Context) (Report, error) {
	doc, err := s.source.Fetch(ctx)
	if err != nil {
		return Report{}, err
	}
	docs, err := split(doc)
	if err != nil {
		s.forget()
		return Report{}, err
	}
	prev := s.active.Load()
	next := &Sites{byHost: map[string]*site.Site{}}
	var rep Report
	for _, d := range docs {
		st, err := s.build(d)
		if err != nil {
			rep.Failed = append(rep.Failed, err)
			if old := prev.byApp(unverifiedApp(d)); old != nil {
				next.add(old)
			}
			continue
		}
		if err := next.add(st); err != nil {
			rep.Failed = append(rep.Failed, err)
		}
	}
	if len(docs) == 1 && next.Len() == 0 && len(rep.Failed) > 0 && prev.single != nil {
		next.single = prev.single // one hand-signed map: keep it on error
	}
	if err := next.check(); err != nil {
		rep.Failed = append(rep.Failed, err)
		next.single = nil
	}
	s.active.Store(next)
	rep.Active = next.All()
	if len(rep.Failed) > 0 {
		s.forget()
		return rep, errors.Join(rep.Failed...)
	}
	return rep, nil
}

func (s *Store) build(doc []byte) (*site.Site, error) {
	m, _, err := Verify(doc, s.pub, s.keyID)
	if err != nil {
		return nil, err
	}
	st, err := site.BuildWith(m, s.opts)
	if err != nil {
		return nil, fmt.Errorf("unusable map of %s: %w", m.Meta.App, err)
	}
	return st, nil
}

// split accepts one signed map or {"items": [...]} (GET /v1/internal/maps).
func split(doc []byte) ([][]byte, error) {
	var list struct {
		Items *[]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(doc, &list); err != nil {
		return nil, fmt.Errorf("unreadable signed document: %w", err)
	}
	if list.Items == nil {
		return [][]byte{doc}, nil
	}
	out := make([][]byte, 0, len(*list.Items))
	for _, d := range *list.Items {
		out = append(out, d)
	}
	return out, nil
}

// unverifiedApp reads meta.app without trusting it: it only selects which
// already verified site to keep when the new map is refused.
func unverifiedApp(doc []byte) string {
	var sm SignedMap
	if json.Unmarshal(doc, &sm) != nil {
		return ""
	}
	payload, err := base64.StdEncoding.DecodeString(sm.Payload)
	if err != nil {
		return ""
	}
	var m struct {
		Meta struct {
			App string `json:"app"`
		} `json:"meta"`
	}
	_ = json.Unmarshal(payload, &m)
	return m.Meta.App
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
