package tunnel

import (
	"sync"
	"time"
)

// Wrong tokens: an address that keeps failing is refused without checking
// for a while, and reported once rather than at every attempt.
const (
	maxFailures   = 20
	failureWindow = time.Minute
	failureKeys   = 10000
)

type failureCount struct {
	n     int
	since time.Time
}

type failures struct {
	mu sync.Mutex
	m  map[string]*failureCount
}

// allowed reports whether an address may still try.
func (f *failures) allowed(addr string, now time.Time) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := f.m[addr]
	return c == nil || now.Sub(c.since) > failureWindow || c.n < maxFailures
}

// failed counts a wrong token; true for the first failure of a window (to
// report it).
func (f *failures) failed(addr string, now time.Time) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.m == nil || len(f.m) >= failureKeys {
		f.m = map[string]*failureCount{}
	}
	c := f.m[addr]
	if c == nil || now.Sub(c.since) > failureWindow {
		f.m[addr] = &failureCount{n: 1, since: now}
		return true
	}
	c.n++
	return false
}
