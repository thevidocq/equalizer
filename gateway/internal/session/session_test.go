package session

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func testManager(t *testing.T, store Store) *Manager {
	t.Helper()
	m, err := NewManager(store, make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func exercise(t *testing.T, store Store) {
	ctx := context.Background()
	m := testManager(t, store)
	p := Policy{Idle: time.Minute, Absolute: time.Hour}
	id := NewID()
	in := &Session{Tenant: "t", App: "a", Role: "citizen", Cookies: map[string]string{"JSESSIONID": "legacy-secret"}}
	if err := m.Save(ctx, id, in, p); err != nil {
		t.Fatal(err)
	}
	got, err := m.Load(ctx, id, "t", "a", p)
	if err != nil || got.Role != "citizen" || got.Cookies["JSESSIONID"] != "legacy-secret" {
		t.Fatalf("load: %+v %v", got, err)
	}
	blob, _ := store.Load(ctx, key(id))
	if strings.Contains(string(blob), "legacy-secret") || strings.Contains(string(blob), "citizen") {
		t.Fatal("the stored session must be encrypted")
	}
	if _, err := m.Load(ctx, id, "other", "a", p); !errors.Is(err, ErrNotFound) {
		t.Errorf("other tenant: %v", err)
	}
	if _, err := m.Load(ctx, NewID(), "t", "a", p); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown id: %v", err)
	}
	if _, err := m.Load(ctx, "not-an-id", "t", "a", p); !errors.Is(err, ErrNotFound) {
		t.Errorf("malformed id: %v", err)
	}
	// A blob moved under another id does not open.
	other := NewID()
	_ = store.Save(ctx, key(other), blob, time.Minute)
	if _, err := m.Load(ctx, other, "t", "a", p); !errors.Is(err, ErrNotFound) {
		t.Errorf("moved blob: %v", err)
	}
	m.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	if _, err := m.Load(ctx, id, "t", "a", p); !errors.Is(err, ErrNotFound) {
		t.Errorf("absolute limit: %v", err)
	}
	m.now = time.Now
	if err := m.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Load(ctx, id, "t", "a", p); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleted: %v", err)
	}
}

func TestMemory(t *testing.T) {
	mem := NewMemory()
	exercise(t, mem)
	_ = mem.Save(context.Background(), "k", []byte("x"), time.Minute)
	mem.now = func() time.Time { return time.Now().Add(2 * time.Minute) }
	if _, err := mem.Load(context.Background(), "k"); !errors.Is(err, ErrNotFound) {
		t.Errorf("idle expiry: %v", err)
	}
}

// TestRedis runs against a real redis-server when one is installed.
func TestRedis(t *testing.T) {
	bin, err := exec.LookPath("redis-server")
	if err != nil {
		t.Skip("redis-server not installed")
	}
	cmd := exec.Command(bin, "--port", "16390", "--save", "", "--appendonly", "no")
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Start(); err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	r, err := NewRedis("redis://127.0.0.1:16390/0")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for i := 0; ; i++ {
		if r.Ping(context.Background()) == nil {
			break
		}
		if i > 50 {
			t.Fatal("redis did not start")
		}
		time.Sleep(50 * time.Millisecond)
	}
	exercise(t, r)
}
