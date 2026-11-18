package budget

import (
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	l := New(1, 20*time.Millisecond)
	if !l.Acquire(t.Context()) {
		t.Fatal("first slot denied")
	}
	start := time.Now()
	if l.Acquire(t.Context()) {
		t.Fatal("second slot granted beyond the limit")
	}
	if time.Since(start) < 20*time.Millisecond {
		t.Fatal("did not wait before refusing")
	}
	l.Release()
	if !l.Acquire(t.Context()) {
		t.Fatal("slot not released")
	}
	if unlimited := New(0, 0); !unlimited.Acquire(t.Context()) || unlimited.InFlight() != 0 {
		t.Fatal("without a limit it must always grant")
	}
}
