package mfa

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

// Box encrypts TOTP secrets at rest (AES-256-GCM). The user id is the
// additional data: a secret copied to another user does not decrypt.
type Box struct{ aead cipher.AEAD }

// NewBox creates a box with a 32-byte key.
func NewBox(key []byte) (*Box, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("MFA key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

func ad(userID int64) []byte { return []byte(fmt.Sprintf("equalizer-mfa-v1:%d", userID)) }

// Seal encrypts a secret for a user.
func (b *Box) Seal(userID int64, secret []byte) []byte {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		panic(err)
	}
	return b.aead.Seal(nonce, nonce, secret, ad(userID))
}

// Open decrypts a secret of a user.
func (b *Box) Open(userID int64, sealed []byte) ([]byte, error) {
	n := b.aead.NonceSize()
	if len(sealed) < n {
		return nil, errors.New("sealed secret too short")
	}
	return b.aead.Open(nil, sealed[:n], sealed[n:], ad(userID))
}

// SealFor encrypts another secret (a tenant's AI key, a test account's
// password). label is the additional data: the secret opens only for it.
func (b *Box) SealFor(label string, secret []byte) []byte {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		panic(err)
	}
	return b.aead.Seal(nonce, nonce, secret, []byte("equalizer-secret-v1:"+label))
}

// OpenFor decrypts a secret sealed with SealFor.
func (b *Box) OpenFor(label string, sealed []byte) ([]byte, error) {
	n := b.aead.NonceSize()
	if len(sealed) < n {
		return nil, errors.New("sealed secret too short")
	}
	return b.aead.Open(nil, sealed[:n], sealed[n:], []byte("equalizer-secret-v1:"+label))
}
