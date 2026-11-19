package session

import (
	"context"
	"time"
)

// ReadOnly wraps a store so that sessions are read but never changed: the
// agent's dry-run tries see the try session as it is, and nothing a refused
// or pretended answer does (a logout, a lost login) is kept.
func ReadOnly(s Store) Store { return readOnly{s} }

type readOnly struct{ s Store }

func (r readOnly) Load(ctx context.Context, key string) ([]byte, error)    { return r.s.Load(ctx, key) }
func (readOnly) Save(context.Context, string, []byte, time.Duration) error { return nil }
func (readOnly) Delete(context.Context, string) error                      { return nil }
