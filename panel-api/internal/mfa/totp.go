// Package mfa implements the second login step: TOTP codes (RFC 6238,
// SHA-1, 6 digits, 30 seconds, as every authenticator app expects),
// recovery codes and the encryption of secrets at rest.
package mfa

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // RFC 6238 default, required by authenticator apps
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Period is the TOTP time step.
const Period = 30 * time.Second

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewSecret returns 20 random bytes.
func NewSecret() []byte {
	s := make([]byte, 20)
	if _, err := rand.Read(s); err != nil {
		panic(err)
	}
	return s
}

// Base32 is the secret as typed into an app.
func Base32(secret []byte) string { return b32.EncodeToString(secret) }

// URI is the otpauth:// link shown as a QR code.
func URI(secret []byte, issuer, account string) string {
	label := url.PathEscape(issuer + ":" + account)
	q := url.Values{"secret": {Base32(secret)}, "issuer": {issuer}, "algorithm": {"SHA1"}, "digits": {"6"}, "period": {"30"}}
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// Step returns the time step of t.
func Step(t time.Time) int64 { return t.Unix() / int64(Period/time.Second) }

// Code computes the 6-digit code of a step.
func Code(secret []byte, step int64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step))
	mac := hmac.New(sha1.New, secret)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	n := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", n%1_000_000)
}

// Verify checks a code against the current step ±1 (clock drift) and
// returns the matched step. Steps at or before lastStep are refused, so a
// code is never accepted twice.
func Verify(secret []byte, code string, now time.Time, lastStep int64) (int64, bool) {
	code = strings.ReplaceAll(strings.TrimSpace(code), " ", "")
	if len(code) != 6 {
		return 0, false
	}
	cur := Step(now)
	for _, s := range []int64{cur - 1, cur, cur + 1} {
		if s <= lastStep {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(Code(secret, s)), []byte(code)) == 1 {
			return s, true
		}
	}
	return 0, false
}

// RecoveryCodes returns n codes like "k7wq-2mzp-9xcd" and their hashes.
func RecoveryCodes(n int) (codes, hashes []string) {
	const alphabet = "abcdefghjkmnpqrstuvwxyz23456789"
	for i := 0; i < n; i++ {
		raw := make([]byte, 12)
		if _, err := rand.Read(raw); err != nil {
			panic(err)
		}
		var b strings.Builder
		for j, c := range raw {
			if j > 0 && j%4 == 0 {
				b.WriteByte('-')
			}
			b.WriteByte(alphabet[int(c)%len(alphabet)])
		}
		codes = append(codes, b.String())
		hashes = append(hashes, HashRecovery(b.String()))
	}
	return codes, hashes
}

// HashRecovery normalises and hashes a recovery code.
func HashRecovery(code string) string {
	norm := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(code), " ", ""))
	sum := sha256.Sum256([]byte(norm))
	return hex.EncodeToString(sum[:])
}
