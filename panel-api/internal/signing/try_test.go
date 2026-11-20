package signing

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"
)

// The gateway's mapstore.TryMessage must build the very same bytes (its
// test checks the same vector).
func TestTryMessageVector(t *testing.T) {
	got := string(TryMessage(time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC), []byte("{}")))
	if want := "equalizer-try/1\n2026-11-19T12:00:00Z\n{}"; got != want {
		t.Fatalf("%q", got)
	}
}

func TestSignTry(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	exp := time.Now().Add(30 * time.Minute)
	tm, err := SignTry([]byte(`{ "a": 1 }`), exp, key, "k1")
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := base64.StdEncoding.DecodeString(tm.Payload)
	sig, _ := base64.StdEncoding.DecodeString(tm.Signature)
	at, _ := time.Parse(time.RFC3339, tm.ExpiresAt)
	if string(payload) != `{"a":1}` || tm.Purpose != "try" || !ed25519.Verify(pub, TryMessage(at, payload), sig) {
		t.Fatalf("%+v", tm)
	}
	if ed25519.Verify(pub, payload, sig) {
		t.Fatal("a try signature verifies as a publish signature")
	}
}
