package budget

import (
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	l := New(1, 20*time.Millisecond)
	if !l.Acquire(t.Context()) {
		t.Fatal("primo posto negato")
	}
	start := time.Now()
	if l.Acquire(t.Context()) {
		t.Fatal("secondo posto concesso oltre il limite")
	}
	if time.Since(start) < 20*time.Millisecond {
		t.Fatal("non ha atteso prima di rifiutare")
	}
	l.Release()
	if !l.Acquire(t.Context()) {
		t.Fatal("posto non liberato")
	}
	if unlimited := New(0, 0); !unlimited.Acquire(t.Context()) || unlimited.InFlight() != 0 {
		t.Fatal("senza limite deve sempre concedere")
	}
}
