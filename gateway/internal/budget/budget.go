// Package budget limita le richieste contemporanee verso il legacy.
//
// Per ora il limite vale per singola replica. Il conteggio condiviso tra
// repliche (Redis) arriverà con il passaggio a più istanze reali.
package budget

import (
	"context"
	"time"
)

// Limiter è un semaforo con attesa massima.
type Limiter struct {
	slots chan struct{}
	wait  time.Duration
}

// New crea un limiter con max richieste contemporanee; max <= 0 significa
// nessun limite.
func New(max int, wait time.Duration) *Limiter {
	if max <= 0 {
		return &Limiter{}
	}
	return &Limiter{slots: make(chan struct{}, max), wait: wait}
}

// Acquire prende un posto, aspettando al massimo il tempo configurato.
// Restituisce false se il budget è esaurito: il chiamante risponde 503.
func (l *Limiter) Acquire(ctx context.Context) bool {
	if l.slots == nil {
		return true
	}
	select {
	case l.slots <- struct{}{}:
		return true
	default:
	}
	t := time.NewTimer(l.wait)
	defer t.Stop()
	select {
	case l.slots <- struct{}{}:
		return true
	case <-t.C:
		return false
	case <-ctx.Done():
		return false
	}
}

// Release libera il posto preso con Acquire.
func (l *Limiter) Release() {
	if l.slots != nil {
		<-l.slots
	}
}

// InFlight restituisce le richieste in corso.
func (l *Limiter) InFlight() int {
	return len(l.slots)
}
