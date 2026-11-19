package mapstore

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// A try signature must never make a map load as published, even when the
// map says published.
func TestTrySignatureIsNotAPublishSignature(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	payload := []byte(`{"meta":{"schema":"gateway-map/1.0","tenant":"t","app":"a","version":1,"status":"published"}}`)
	exp := time.Now().Add(time.Minute).UTC().Truncate(time.Second)
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(key, TryMessage(exp, payload)))
	tm := TryMap{Payload: base64.StdEncoding.EncodeToString(payload), Signature: sig, KeyID: "k", Purpose: "try", ExpiresAt: exp}
	if _, _, err := VerifyTry(tm, pub, "k", time.Now()); err != nil {
		t.Fatalf("try map refused: %v", err)
	}
	doc, _ := json.Marshal(SignedMap{Version: 1, Payload: tm.Payload, Signature: sig, KeyID: "k"})
	if _, _, err := Verify(doc, pub, "k"); !errors.Is(err, ErrBadSignature) {
		t.Errorf("try signature accepted as a published map: %v", err)
	}
	// The expiry is signed: moving it breaks the signature.
	tm.ExpiresAt = exp.Add(time.Minute)
	if _, _, err := VerifyTry(tm, pub, "k", time.Now()); !errors.Is(err, ErrBadSignature) {
		t.Errorf("moved expiry accepted: %v", err)
	}
}

// panel-api's signing.TryMessage checks the same vector.
func TestTryMessageVector(t *testing.T) {
	got := string(TryMessage(time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC), []byte("{}")))
	if want := "equalizer-try/1\n2026-11-19T12:00:00Z\n{}"; got != want {
		t.Fatalf("%q", got)
	}
}
