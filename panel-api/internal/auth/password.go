// Package auth handles panel users' passwords and sessions.
package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Iterations of PBKDF2-HMAC-SHA256 (OWASP recommendation, 2023).
const Iterations = 600_000

// MinPasswordLength applies to new passwords.
const MinPasswordLength = 12

// ErrWeakPassword reports a password that is too short or too long.
var ErrWeakPassword = fmt.Errorf("the password must have %d to 256 characters", MinPasswordLength)

// HashPassword returns "pbkdf2-sha256$<iterations>$<salt>$<hash>".
func HashPassword(password string) (string, error) {
	if n := utf8.RuneCountInString(password); n < MinPasswordLength || len(password) > 256 {
		return "", ErrWeakPassword
	}
	return hashWith(password, Iterations)
}

func hashWith(password string, iter int) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, iter, 32)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", iter,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

// CheckPassword compares a password with a stored hash in constant time.
func CheckPassword(stored, password string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" || len(password) > 256 {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter < 1 {
		return false
	}
	salt, err1 := base64.RawStdEncoding.DecodeString(parts[2])
	want, err2 := base64.RawStdEncoding.DecodeString(parts[3])
	if err := errors.Join(err1, err2); err != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iter, len(want))
	return err == nil && subtle.ConstantTimeCompare(got, want) == 1
}

// dummyHash is checked when the user does not exist, so the answer takes
// the same time and does not reveal which emails are registered.
var dummyHash, _ = hashWith("equalizer-dummy-password", Iterations)

// CheckDummy spends the time of a password check.
func CheckDummy(password string) { CheckPassword(dummyHash, password) }
