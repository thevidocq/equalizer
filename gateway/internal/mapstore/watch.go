package mapstore

import (
	"context"
	"time"

	"github.com/thevidocq/equalizer/gateway/internal/site"
)

// Watch checks the source every interval, and at once when a value arrives
// on trigger (e.g. SIGHUP, which forces a full reload). onResult receives the
// outcome of every attempt that is not "unchanged".
func (s *Store) Watch(ctx context.Context, interval time.Duration, trigger <-chan struct{}, onResult func(*site.Site, error)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-trigger:
			s.forget()
		case <-ticker.C:
		}
		fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		a, err := s.ReloadContext(fetchCtx)
		cancel()
		if IsUnchanged(err) {
			continue
		}
		onResult(a, err)
	}
}
