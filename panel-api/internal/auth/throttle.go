package auth

import (
	"sync"
	"time"
)

// Throttle limits failed logins per key (IP and email) in this process:
// after Max failures within Window, logins are refused until the window ends.
type Throttle struct {
	Max    int
	Window time.Duration
	mu     sync.Mutex
	fails  map[string][]time.Time
}

// NewThrottle creates a throttle.
func NewThrottle(max int, window time.Duration) *Throttle {
	return &Throttle{Max: max, Window: window, fails: map[string][]time.Time{}}
}

func (t *Throttle) recent(key string, now time.Time) []time.Time {
	var keep []time.Time
	for _, ts := range t.fails[key] {
		if now.Sub(ts) < t.Window {
			keep = append(keep, ts)
		}
	}
	if len(keep) == 0 {
		delete(t.fails, key)
	} else {
		t.fails[key] = keep
	}
	return keep
}

// Blocked reports whether any key is over the limit.
func (t *Throttle) Blocked(keys ...string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	for _, k := range keys {
		if len(t.recent(k, now)) >= t.Max {
			return true
		}
	}
	return false
}

// Fail records a failure for every key.
func (t *Throttle) Fail(keys ...string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	if len(t.fails) > 100000 {
		for k := range t.fails {
			t.recent(k, now)
		}
	}
	for _, k := range keys {
		t.fails[k] = append(t.recent(k, now), now)
	}
}

// Reset forgets the failures of the keys (after a successful login).
func (t *Throttle) Reset(keys ...string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, k := range keys {
		delete(t.fails, k)
	}
}
