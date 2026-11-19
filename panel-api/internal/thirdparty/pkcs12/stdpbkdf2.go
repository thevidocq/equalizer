package pkcs12

import (
	"crypto/pbkdf2"
	"hash"
)

// pbkdf2Key adapts the standard library's PBKDF2 (Go 1.24) to the call of
// golang.org/x/crypto/pbkdf2 that the upstream code makes (see README).
func pbkdf2Key(password, salt []byte, iter, keyLen int, h func() hash.Hash) []byte {
	k, err := pbkdf2.Key(h, string(password), salt, iter, keyLen)
	if err != nil {
		return nil
	}
	return k
}
