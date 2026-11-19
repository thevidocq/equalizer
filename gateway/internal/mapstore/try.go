package mapstore

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
)

// TryPurpose is the only purpose a try map can be signed for.
const TryPurpose = "try"

// MaxTryLife bounds how far in the future a try map may expire: a leaked
// one is useless soon.
const MaxTryLife = 2 * time.Hour

// TryMap is a draft signed by panel-api for the agent's tries
// (TryRequest.map in schema/api/gateway-admin.yaml).
type TryMap struct {
	Payload   string    `json:"payload"`
	Signature string    `json:"signature"`
	KeyID     string    `json:"key_id"`
	Purpose   string    `json:"purpose"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Try verification errors.
var (
	ErrTryPurpose = errors.New("map not signed for try")
	ErrTryExpired = errors.New("try map expired or expiring too far ahead")
)

// TryMessage is what panel-api signs for a try map. The prefix keeps a try
// signature from ever verifying as a published map's (those sign the bare
// payload), and the expiry is part of what is signed.
func TryMessage(expiresAt time.Time, payload []byte) []byte {
	head := "equalizer-try/1\n" + expiresAt.UTC().Format(time.RFC3339) + "\n"
	return append([]byte(head), payload...)
}

// VerifyTry checks a try map and returns its model and payload. Any status
// is accepted (it is usually a draft); everything else is checked like a
// published map.
func VerifyTry(t TryMap, pub ed25519.PublicKey, expectedKeyID string, now time.Time) (*mapmodel.Model, []byte, error) {
	if t.Purpose != TryPurpose {
		return nil, nil, ErrTryPurpose
	}
	if expectedKeyID != "" && t.KeyID != expectedKeyID {
		return nil, nil, ErrWrongKey
	}
	if !now.Before(t.ExpiresAt) || t.ExpiresAt.After(now.Add(MaxTryLife)) {
		return nil, nil, ErrTryExpired
	}
	payload, err := base64.StdEncoding.DecodeString(t.Payload)
	if err != nil {
		return nil, nil, fmt.Errorf("payload is not base64: %w", err)
	}
	sig, err := base64.StdEncoding.DecodeString(t.Signature)
	if err != nil {
		return nil, nil, fmt.Errorf("signature is not base64: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize || !ed25519.Verify(pub, TryMessage(t.ExpiresAt, payload), sig) {
		return nil, nil, ErrBadSignature
	}
	var m mapmodel.Model
	if err := json.Unmarshal(payload, &m); err != nil {
		return nil, nil, fmt.Errorf("unreadable map: %w", err)
	}
	if m.Meta.Schema != SchemaVersion {
		return nil, nil, fmt.Errorf("%w: %q", ErrSchemaVersion, m.Meta.Schema)
	}
	return &m, payload, nil
}
