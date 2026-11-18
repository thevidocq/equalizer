// Package limits applies the map's rate limits and behaviour rules with
// counters shared by every gateway replica (Redis), or in process.
package limits

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Store keeps windowed counters, sets and marks.
type Store interface {
	// Incr adds one to key and returns the new value; the key lives ttl.
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)
	// AddDistinct adds member to the set at key and returns its size.
	AddDistinct(ctx context.Context, key, member string, ttl time.Duration) (int64, error)
	// Mark sets key for ttl (a block or a throttle).
	Mark(ctx context.Context, key string, ttl time.Duration) error
	// Marked returns the remaining time of the first marked key (0 if none).
	Marked(ctx context.Context, keys ...string) (time.Duration, error)
}

// Redis is a Store shared by the replicas.
type Redis struct{ c *redis.Client }

// NewRedis uses an existing client (the one of the session store).
func NewRedis(c *redis.Client) *Redis { return &Redis{c: c} }

var incrScript = redis.NewScript(`
local n = redis.call("INCR", KEYS[1])
if n == 1 then redis.call("PEXPIRE", KEYS[1], ARGV[1]) end
return n`)

var addScript = redis.NewScript(`
redis.call("SADD", KEYS[1], ARGV[1])
if redis.call("PTTL", KEYS[1]) < 0 then redis.call("PEXPIRE", KEYS[1], ARGV[2]) end
return redis.call("SCARD", KEYS[1])`)

// Incr implements Store.
func (r *Redis) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	return incrScript.Run(ctx, r.c, []string{key}, ttl.Milliseconds()).Int64()
}

// AddDistinct implements Store.
func (r *Redis) AddDistinct(ctx context.Context, key, member string, ttl time.Duration) (int64, error) {
	return addScript.Run(ctx, r.c, []string{key}, member, ttl.Milliseconds()).Int64()
}

// Mark implements Store.
func (r *Redis) Mark(ctx context.Context, key string, ttl time.Duration) error {
	return r.c.Set(ctx, key, "1", ttl).Err()
}

// Marked implements Store.
func (r *Redis) Marked(ctx context.Context, keys ...string) (time.Duration, error) {
	if len(keys) == 0 {
		return 0, nil
	}
	pipe := r.c.Pipeline()
	cmds := make([]*redis.DurationCmd, len(keys))
	for i, k := range keys {
		cmds[i] = pipe.PTTL(ctx, k)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	for _, c := range cmds {
		if d := c.Val(); d > 0 {
			return d, nil
		}
	}
	return 0, nil
}

// Memory is an in-process Store (tests, single replica).
type Memory struct {
	mu    sync.Mutex
	items map[string]*memItem
	now   func() time.Time
}

type memItem struct {
	n       int64
	set     map[string]bool
	expires time.Time
}

// NewMemory creates an empty in-process store.
func NewMemory() *Memory { return &Memory{items: map[string]*memItem{}, now: time.Now} }

func (m *Memory) get(key string, ttl time.Duration) *memItem {
	now := m.now()
	it, ok := m.items[key]
	if !ok || now.After(it.expires) {
		if len(m.items) > 200000 {
			for k, v := range m.items {
				if now.After(v.expires) {
					delete(m.items, k)
				}
			}
		}
		it = &memItem{expires: now.Add(ttl)}
		m.items[key] = it
	}
	return it
}

// Incr implements Store.
func (m *Memory) Incr(_ context.Context, key string, ttl time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	it := m.get(key, ttl)
	it.n++
	return it.n, nil
}

// AddDistinct implements Store.
func (m *Memory) AddDistinct(_ context.Context, key, member string, ttl time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	it := m.get(key, ttl)
	if it.set == nil {
		it.set = map[string]bool{}
	}
	it.set[member] = true
	return int64(len(it.set)), nil
}

// Mark implements Store.
func (m *Memory) Mark(_ context.Context, key string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[key] = &memItem{n: 1, expires: m.now().Add(ttl)}
	return nil
}

// Marked implements Store.
func (m *Memory) Marked(_ context.Context, keys ...string) (time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	for _, k := range keys {
		if it, ok := m.items[k]; ok && now.Before(it.expires) {
			return it.expires.Sub(now), nil
		}
	}
	return 0, nil
}
