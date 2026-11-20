// Package signing produces the SignedMap document read by the gateway
// (schema/api/panel-api.yaml, SignedMap).
package signing

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"time"
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

// TryMap is a draft signed for the agent's tries on the gateway
// (TryRequest.map in schema/api/gateway-admin.yaml). It never loads as a
// published map: the signed message differs.
type TryMap struct {
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
	KeyID     string `json:"key_id"`
	Purpose   string `json:"purpose"`
	ExpiresAt string `json:"expires_at"`
}

// TryMessage is the signed message of a try map; the gateway builds the
// same one (mapstore.TryMessage).
func TryMessage(expiresAt time.Time, payload []byte) []byte {
	head := "equalizer-try/1\n" + expiresAt.UTC().Format(time.RFC3339) + "\n"
	return append([]byte(head), payload...)
}

// SignTry compacts a draft and signs it for tries until expiresAt (at most
// two hours ahead: the gateway refuses longer).
func SignTry(doc []byte, expiresAt time.Time, key ed25519.PrivateKey, keyID string) (TryMap, error) {
	var payload bytes.Buffer
	if err := json.Compact(&payload, doc); err != nil {
		return TryMap{}, err
	}
	exp := expiresAt.UTC().Truncate(time.Second)
	return TryMap{
		Payload:   base64.StdEncoding.EncodeToString(payload.Bytes()),
		Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(key, TryMessage(exp, payload.Bytes()))),
		KeyID:     keyID,
		Purpose:   "try",
		ExpiresAt: exp.Format(time.RFC3339),
	}, nil
}
