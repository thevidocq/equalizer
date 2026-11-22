package server

import "sync"

// maxStreamsPerClient bounds the audio and video streams one visitor holds.
const maxStreamsPerClient = 2

// mediaClients counts the streams each visitor holds, per site.
type mediaClients struct {
	mu sync.Mutex
	n  map[string]int
}

func (c *mediaClients) take(app, ip string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.n == nil {
		c.n = map[string]int{}
	}
	k := app + "\x00" + ip
	if c.n[k] >= maxStreamsPerClient {
		return false
	}
	c.n[k]++
	return true
}

func (c *mediaClients) give(app, ip string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	k := app + "\x00" + ip
	if c.n[k] <= 1 {
		delete(c.n, k)
		return
	}
	c.n[k]--
}
