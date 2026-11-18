package mapstore

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
)

// SchemaVersion is the only map schema version accepted by this gateway.
const SchemaVersion = "gateway-map/1.0"

// SignedMap is the signed document produced by the panel
// (components.schemas.SignedMap in schema/api/panel-api.yaml).
type SignedMap struct {
	Version   int    `json:"version"`
	Payload   string `json:"payload"`   // base64 of the map's compact JSON
	Signature string `json:"signature"` // base64 of the Ed25519 signature of the payload bytes
	KeyID     string `json:"key_id"`
}

// Verification errors, distinguishable in logs.
var (
	ErrBadSignature  = errors.New("invalid map signature")
	ErrWrongKey      = errors.New("map signed with a key other than the expected one")
	ErrSchemaVersion = errors.New("unsupported map schema version")
	ErrNotPublished  = errors.New("map status is not published")
	ErrVersionMatch  = errors.New("signed document version differs from meta.version")
)

// Verify checks signature and consistency and returns the decoded map.
// An empty expectedKeyID accepts any id; the public key is still binding.
func Verify(doc []byte, pub ed25519.PublicKey, expectedKeyID string) (*mapmodel.Model, []byte, error) {
	var sm SignedMap
	if err := json.Unmarshal(doc, &sm); err != nil {
		return nil, nil, fmt.Errorf("unreadable signed document: %w", err)
	}
	if expectedKeyID != "" && sm.KeyID != expectedKeyID {
		return nil, nil, ErrWrongKey
	}
	payload, err := base64.StdEncoding.DecodeString(sm.Payload)
	if err != nil {
		return nil, nil, fmt.Errorf("payload is not base64: %w", err)
	}
	sig, err := base64.StdEncoding.DecodeString(sm.Signature)
	if err != nil {
		return nil, nil, fmt.Errorf("signature is not base64: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize || !ed25519.Verify(pub, payload, sig) {
		return nil, nil, ErrBadSignature
	}

	var m mapmodel.Model
	if err := json.Unmarshal(payload, &m); err != nil {
		return nil, nil, fmt.Errorf("unreadable map: %w", err)
	}
	switch {
	case m.Meta.Schema != SchemaVersion:
		return nil, nil, fmt.Errorf("%w: %q", ErrSchemaVersion, m.Meta.Schema)
	case m.Meta.Status != "published":
		return nil, nil, ErrNotPublished
	case m.Meta.Version != sm.Version:
		return nil, nil, ErrVersionMatch
	}
	return &m, payload, nil
}

// ParsePublicKey decodes a base64 Ed25519 public key.
func ParsePublicKey(b64 string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("public key is not base64: %w", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("public key has %d bytes, want %d", len(raw), ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(raw), nil
}
