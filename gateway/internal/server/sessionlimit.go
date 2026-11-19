package server

import (
	"sync"
	"time"
)

// Every visitor without a session whose page sets a legacy cookie gets a
// new session in the store (Joomla sets one at once). A bot sending
// thousands of requests would fill the store and push out the sessions of
// real visitors. So a new anonymous session lives freshIdle only until the
// visitor comes back with its cookie, and each address may create only so
// many: enough for an office or a mobile carrier behind one address. Past
// the limit the page is still served, without a session.
const (
	freshIdle            = 5 * time.Minute
	newSessionsPerMinute = 600
	newSessionsBurst     = 300
	newSessionsKeys      = 100000 // addresses remembered by one gateway
)

var anonSessions = &keyLimiter{rate: newSessionsPerMinute / 60.0, burst: newSessionsBurst, max: newSessionsKeys,
	buckets: map[string]*bucket{}}

type bucket struct {
	tokens float64
	last   time.Time
	warned bool // the first refusal since the last allowed session was reported
}

// keyLimiter is a token bucket per key, in the memory of this gateway.
type keyLimiter struct {
	mu      sync.Mutex
	rate    float64 // tokens per second
	burst   float64
	max     int
	buckets map[string]*bucket
}

func (l *keyLimiter) allow(key string, now time.Time) bool {
	ok, _ := l.check(key, now)
	return ok
}

// check is allow, also telling whether this is the first refusal since the
// key was last allowed (reported once, not at every request of a flood).
func (l *keyLimiter) check(key string, now time.Time) (ok, first bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, found := l.buckets[key]
	if !found {
		if len(l.buckets) >= l.max {
			l.forget(now)
		}
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	b.tokens = min(l.burst, b.tokens+now.Sub(b.last).Seconds()*l.rate)
	b.last = now
	if b.tokens < 1 {
		first = !b.warned
		b.warned = true
		return false, first
	}
	b.tokens--
	b.warned = false
	return true, false
}

// forget drops the buckets that are full again (no recent use); if every
// key is busy, all are dropped rather than growing without end.
func (l *keyLimiter) forget(now time.Time) {
	for k, b := range l.buckets {
		if b.tokens+now.Sub(b.last).Seconds()*l.rate >= l.burst {
			delete(l.buckets, k)
		}
	}
	if len(l.buckets) >= l.max {
		l.buckets = map[string]*bucket{}
	}
}
