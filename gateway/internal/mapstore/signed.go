package mapstore

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/thevidocq/equalizer/gateway/internal/mapmodel"
)

// SchemaVersion è l'unica versione di schema accettata da questo gateway.
const SchemaVersion = "gateway-map/1.0"

// SignedMap è il documento firmato prodotto dal pannello
// (components.schemas.SignedMap in schema/api/panel-api.yaml).
type SignedMap struct {
	Version   int    `json:"version"`
	Payload   string `json:"payload"`   // base64 del JSON compatto della mappa
	Signature string `json:"signature"` // base64 della firma Ed25519 dei byte di payload
	KeyID     string `json:"key_id"`
}

// Errori di verifica, distinguibili nei log.
var (
	ErrBadSignature  = errors.New("firma della mappa non valida")
	ErrWrongKey      = errors.New("mappa firmata con una chiave diversa da quella attesa")
	ErrSchemaVersion = errors.New("versione di schema della mappa non supportata")
	ErrNotPublished  = errors.New("la mappa non è in stato published")
	ErrVersionMatch  = errors.New("versione del documento firmato diversa da meta.version")
)

// Verify controlla firma e coerenza e restituisce la mappa decodificata.
// expectedKeyID vuoto significa "qualunque id": la chiave pubblica resta vincolante.
func Verify(doc []byte, pub ed25519.PublicKey, expectedKeyID string) (*mapmodel.Model, []byte, error) {
	var sm SignedMap
	if err := json.Unmarshal(doc, &sm); err != nil {
		return nil, nil, fmt.Errorf("documento firmato non leggibile: %w", err)
	}
	if expectedKeyID != "" && sm.KeyID != expectedKeyID {
		return nil, nil, ErrWrongKey
	}
	payload, err := base64.StdEncoding.DecodeString(sm.Payload)
	if err != nil {
		return nil, nil, fmt.Errorf("payload non in base64: %w", err)
	}
	sig, err := base64.StdEncoding.DecodeString(sm.Signature)
	if err != nil {
		return nil, nil, fmt.Errorf("firma non in base64: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize || !ed25519.Verify(pub, payload, sig) {
		return nil, nil, ErrBadSignature
	}

	var m mapmodel.Model
	if err := json.Unmarshal(payload, &m); err != nil {
		return nil, nil, fmt.Errorf("mappa non leggibile: %w", err)
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

// ParsePublicKey decodifica una chiave pubblica Ed25519 in base64.
func ParsePublicKey(b64 string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("chiave pubblica non in base64: %w", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("chiave pubblica di %d byte, attesi %d", len(raw), ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(raw), nil
}
