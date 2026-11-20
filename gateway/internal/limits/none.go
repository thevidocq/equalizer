package limits

import (
	"context"
	"time"
)

// None is a Store that never counts: every limit and behaviour rule sees a
// first occurrence. The agent's tries use it, so the attack set measures the
// map's types and rules, not the rate limits, and real visitors' counters
// are never touched.
type None struct{}

// Incr always returns 1.
func (None) Incr(context.Context, string, time.Duration) (int64, error) { return 1, nil }

// AddDistinct always returns 1.
func (None) AddDistinct(context.Context, string, string, time.Duration) (int64, error) {
	return 1, nil
}

// Mark keeps nothing.
func (None) Mark(context.Context, string, time.Duration) error { return nil }

// Marked never finds a mark.
func (None) Marked(context.Context, ...string) (time.Duration, error) { return 0, nil }
