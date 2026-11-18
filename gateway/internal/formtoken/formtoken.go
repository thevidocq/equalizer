// Package formtoken seals the server-side state of a rendered form into an
// opaque token placed in the page as the only hidden field.
//
// The token is encrypted and authenticated (AES-256-GCM) with a key shared by
// all gateway replicas, so no local state is needed. It carries:
//   - the declared form it belongs to and the path parameters of its action;
//   - the legacy hidden fields the map keeps ({{hidden.x}}), which therefore
//     never reach the browser;
//   - the encoding of the legacy page, to re-encode the submitted values;
//   - a hash of the browser's binding cookie (CSRF protection: a token taken
//     from another browser is useless) and an expiry.
package formtoken

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Field is the name of the hidden input that carries the token.
const Field = "_eq_token"

// KeySize is the key length in bytes.
const KeySize = 32

// DefaultTTL is how long a rendered form stays valid.
const DefaultTTL = 2 * time.Hour

// Errors returned by Open. They are reasons, never details about the content.
var (
	ErrInvalid = errors.New("form token invalid")
	ErrExpired = errors.New("form token expired")
	ErrBinding = errors.New("form token bound to another browser")
)

// additional data: changing the token format must change this value.
var aad = []byte("equalizer-form-v1")

// State is what the token carries.
type State struct {
	Form    string            `json:"f"`
	Params  map[string]string `json:"p,omitempty"`
	Hidden  map[string]string `json:"h,omitempty"`
	Charset string            `json:"c"`
	Binding string            `json:"b"`
	Expires int64             `json:"x"`
}

// Sealer seals and opens tokens. It is safe for concurrent use.
type Sealer struct {
	aead cipher.AEAD
	ttl  time.Duration
	now  func() time.Time
}

// New creates a sealer with a 32-byte key.
func New(key []byte) (*Sealer, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("form key must be %d bytes, got %d", KeySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Sealer{aead: aead, ttl: DefaultTTL, now: time.Now}, nil
}

// RandomKey returns a new random key.
func RandomKey() []byte {
	k := make([]byte, KeySize)
	if _, err := rand.Read(k); err != nil {
		panic(err) // the system's random source does not fail in practice
	}
	return k
}

// Binding returns the value stored in the token for a binding cookie.
func Binding(cookie string) string {
	sum := sha256.Sum256([]byte(cookie))
	return base64.RawURLEncoding.EncodeToString(sum[:16])
}

// Seal encrypts the state. Expires and Binding are set here.
func (s *Sealer) Seal(st State, cookie string) (string, error) {
	st.Expires = s.now().Add(s.ttl).Unix()
	st.Binding = Binding(cookie)
	plain, err := json.Marshal(st)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := s.aead.Seal(nonce, nonce, plain, aad)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

// Open decrypts and checks a token against the browser's binding cookie.
func (s *Sealer) Open(token, cookie string) (State, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) < s.aead.NonceSize()+s.aead.Overhead() {
		return State{}, ErrInvalid
	}
	n := s.aead.NonceSize()
	plain, err := s.aead.Open(nil, raw[:n], raw[n:], aad)
	if err != nil {
		return State{}, ErrInvalid
	}
	var st State
	if err := json.Unmarshal(plain, &st); err != nil || st.Form == "" {
		return State{}, ErrInvalid
	}
	if s.now().Unix() > st.Expires {
		return State{}, ErrExpired
	}
	if cookie == "" || subtle.ConstantTimeCompare([]byte(st.Binding), []byte(Binding(cookie))) != 1 {
		return State{}, ErrBinding
	}
	return st, nil
}
