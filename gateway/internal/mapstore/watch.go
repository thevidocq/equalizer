package mapstore

import (
	"context"
	"os"
	"time"
)

// Watch ricarica la mappa quando il file cambia (controllo ogni interval)
// o quando arriva un valore su trigger (es. SIGHUP). onResult riceve
// l'esito di ogni tentativo.
func (s *Store) Watch(ctx context.Context, interval time.Duration, trigger <-chan struct{}, onResult func(*Active, error)) {
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
