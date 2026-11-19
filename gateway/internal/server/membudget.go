package server

import "sync"

// memBudget bounds the request bodies held in memory at once, across all
// requests: a few large forms or JSON bodies at a time are fine, thousands
// sent together would exhaust the memory of the gateway. Uploads do not
// count: they are rebuilt into temporary files.
type memBudget struct {
	mu        sync.Mutex
	used, max int64
}

// bodyMemory is shared by every site the gateway serves.
var bodyMemory = &memBudget{max: 256 << 20}

// take reserves n bytes; false when the budget is spent (the request is
// refused with 503, the client may retry).
func (b *memBudget) take(n int64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.used+n > b.max {
		return false
	}
	b.used += n
	return true
}

func (b *memBudget) give(n int64) {
	b.mu.Lock()
	b.used -= n
	b.mu.Unlock()
}
