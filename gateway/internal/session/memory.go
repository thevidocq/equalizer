package session

import (
	"context"
	"sync"
	"time"
)

// Memory is an in-process store, for tests and a single replica without Redis.
type Memory struct {
	mu    sync.Mutex
	items map[string]memItem
	now   func() time.Time
}

type memItem struct {
	blob    []byte
	expires time.Time
}

// NewMemory creates an empty in-process store.
func NewMemory() *Memory { return &Memory{items: map[string]memItem{}, now: time.Now} }

// Load implements Store.
func (m *Memory) Load(_ context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	it, ok := m.items[key]
	if !ok || m.now().After(it.expires) {
		delete(m.items, key)
		return nil, ErrNotFound
	}
	return append([]byte(nil), it.blob...), nil
}

// Save implements Store.
func (m *Memory) Save(_ context.Context, key string, blob []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.items) > 100000 {
		m.sweep()
	}
	m.items[key] = memItem{blob: append([]byte(nil), blob...), expires: m.now().Add(ttl)}
	return nil
}

// Delete implements Store.
func (m *Memory) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, key)
	return nil
}

func (m *Memory) sweep() {
	now := m.now()
	for k, it := range m.items {
		if now.After(it.expires) {
			delete(m.items, k)
		}
	}
}
