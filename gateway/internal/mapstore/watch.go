package mapstore

import (
	"context"
	"os"
	"time"

	"github.com/thevidocq/equalizer/gateway/internal/site"
)

// Watch reloads the map when the file changes (checked every interval) or
// when a value arrives on trigger (e.g. SIGHUP). onResult receives the
// outcome of every attempt.
func (s *Store) Watch(ctx context.Context, interval time.Duration, trigger <-chan struct{}, onResult func(*site.Site, error)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	last := s.modTime()
	for {
		select {
		case <-ctx.Done():
			return
		case <-trigger:
		case <-ticker.C:
			mt := s.modTime()
			if mt.Equal(last) {
				continue
			}
			last = mt
		}
		a, err := s.Reload()
		onResult(a, err)
	}
}

func (s *Store) modTime() time.Time {
	fi, err := os.Stat(s.path)
	if err != nil {
		return time.Time{}
	}
	return fi.ModTime()
}
