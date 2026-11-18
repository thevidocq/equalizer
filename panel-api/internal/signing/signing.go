// Package signing produces the SignedMap document read by the gateway
// (schema/api/panel-api.yaml, SignedMap).
package signing

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
)

// SignedMap is the signed document.
type SignedMap struct {
	Version   int    `json:"version"`
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
	KeyID     string `json:"key_id"`
}

// Sign compacts the document and signs its exact bytes.
func Sign(doc []byte, version int, key ed25519.PrivateKey, keyID string) (SignedMap, error) {
	var payload bytes.Buffer
	if err := json.Compact(&payload, doc); err != nil {
		return SignedMap{}, err
	}
	return SignedMap{
		Version:   version,
		Payload:   base64.StdEncoding.EncodeToString(payload.Bytes()),
		Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(key, payload.Bytes())),
		KeyID:     keyID,
	}, nil
}
