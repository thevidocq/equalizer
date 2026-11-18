// Package unmapped counts the requests refused because their path is not in
// the map and sends the counts to the panel once a minute. Discovery uses
// them to find what real visitors miss. Only the path is kept (never the
// query), and client addresses are only counted, never sent.
package unmapped

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Limits keep memory bounded whatever a scanner sends.
const (
	MaxEntries    = 1000
	MaxPath       = 512
	maxClientKeys = 64
)

type key struct{ app, method, path string }

type entry struct {
	hits    int
	clients map[[8]byte]struct{}
}

// Reporter collects and sends counts. A nil Reporter does nothing.
type Reporter struct {
	URL, Token string
	Client     *http.Client
	Log        *slog.Logger
	mu         sync.Mutex
	entries    map[key]*entry
}

// Record counts one refused request.
func (r *Reporter) Record(app, method, path, clientIP string) {
	if r == nil || app == "" || !strings.HasPrefix(path, "/") {
		return
	}
	if len(path) > MaxPath {
		path = path[:MaxPath]
	}
	k := key{app, method, path}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.entries == nil {
		r.entries = map[key]*entry{}
	}
	e := r.entries[k]
	if e == nil {
		if len(r.entries) >= MaxEntries {
			return
		}
		e = &entry{clients: map[[8]byte]struct{}{}}
		r.entries[k] = e
	}
	e.hits++
	if len(e.clients) < maxClientKeys {
		sum := sha256.Sum256([]byte(clientIP))
		var h [8]byte
		copy(h[:], sum[:8])
		e.clients[h] = struct{}{}
	}
}

type item struct {
	App     string `json:"app"`
	Method  string `json:"method"`
	Path    string `json:"path"`
	Hits    int    `json:"hits"`
	Clients int    `json:"clients"`
}

// take empties the counters and returns them.
func (r *Reporter) take() []item {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]item, 0, len(r.entries))
	for k, e := range r.entries {
		out = append(out, item{App: k.app, Method: k.method, Path: k.path, Hits: e.hits, Clients: len(e.clients)})
	}
	r.entries = nil
	return out
}

// Flush sends the counts; on error they are dropped (they are hints).
func (r *Reporter) Flush(ctx context.Context) error {
	if r == nil {
		return nil
	}
	items := r.take()
	if len(items) == 0 {
		return nil
	}
	body, _ := json.Marshal(map[string]any{"items": items})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.Client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("panel answered %d", resp.StatusCode)
	}
	return nil
}

// Run flushes every interval until ctx ends.
func (r *Reporter) Run(ctx context.Context, every time.Duration) {
	if r == nil {
		return
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			fctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			if err := r.Flush(fctx); err != nil && r.Log != nil {
				r.Log.Warn("unmapped paths not reported", slog.String("error", err.Error()))
			}
			cancel()
		}
	}
}
