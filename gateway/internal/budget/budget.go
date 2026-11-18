// Package budget limits concurrent requests to the legacy application.
//
// For now the limit applies per replica. A count shared across replicas
// (Redis) will come with multiple real instances.
package budget

import (
	"context"
	"time"
)

// Limiter is a semaphore with a maximum wait.
type Limiter struct {
	slots chan struct{}
	wait  time.Duration
}

// New creates a limiter with max concurrent requests; max <= 0 means no limit.
func New(max int, wait time.Duration) *Limiter {
	if max <= 0 {
		return &Limiter{}
	}
	return &Limiter{slots: make(chan struct{}, max), wait: wait}
}

// Acquire takes a slot, waiting at most the configured time. It returns
// false when the budget is exhausted: the caller answers 503.
func (l *Limiter) Acquire(ctx context.Context) bool {
	if l.slots == nil {
		return true
	}
	select {
	case l.slots <- struct{}{}:
		return true
	default:
	}
	t := time.NewTimer(l.wait)
	defer t.Stop()
	select {
	case l.slots <- struct{}{}:
		return true
	case <-t.C:
		return false
	case <-ctx.Done():
		return false
	}
}

// Release frees the slot taken with Acquire.
func (l *Limiter) Release() {
	if l.slots != nil {
		<-l.slots
	}
}

// InFlight returns the number of requests in progress.
func (l *Limiter) InFlight() int {
	return len(l.slots)
}
