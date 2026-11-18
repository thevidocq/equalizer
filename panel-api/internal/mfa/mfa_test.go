package mfa

import (
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func TestRFC6238Vectors(t *testing.T) {
	// RFC 6238, appendix B, SHA-1 seed; codes truncated to 6 digits.
	secret := []byte("12345678901234567890")
	cases := map[int64]string{59: "287082", 1111111109: "081804", 1234567890: "005924", 2000000000: "279037"}
	for ts, want := range cases {
		if got := Code(secret, Step(time.Unix(ts, 0))); got != want {
			t.Errorf("t=%d: %s, want %s", ts, got, want)
		}
	}
}

func TestVerify(t *testing.T) {
	secret := NewSecret()
	now := time.Now()
	code := Code(secret, Step(now))
	step, ok := Verify(secret, code[:3]+" "+code[3:], now, 0)
	if !ok || step != Step(now) {
		t.Fatal("current code refused")
	}
	if _, ok := Verify(secret, code, now, step); ok {
		t.Fatal("a code was accepted twice")
	}
	if _, ok := Verify(secret, Code(secret, Step(now)-1), now, 0); !ok {
		t.Error("previous step refused (clock drift)")
	}
	if _, ok := Verify(secret, Code(secret, Step(now)-3), now, 0); ok {
		t.Error("old code accepted")
	}
	if _, ok := Verify(secret, "12345", now, 0); ok {
		t.Error("short code accepted")
	}
	if !strings.HasPrefix(URI(secret, "Equalizer", "a@b.c"), "otpauth://totp/Equalizer:a@b.c?") {
		t.Error(URI(secret, "Equalizer", "a@b.c"))
	}
}

func TestRecoveryAndBox(t *testing.T) {
	codes, hashes := RecoveryCodes(10)
	if len(codes) != 10 || len(codes[0]) != 14 || HashRecovery(" "+strings.ToUpper(codes[0])+" ") != hashes[0] {
		t.Fatalf("recovery codes: %v", codes[:2])
	}
	key, _ := hex.DecodeString(strings.Repeat("ab", 32))
	box, _ := NewBox(key)
	sealed := box.Seal(7, []byte("secret"))
	if got, err := box.Open(7, sealed); err != nil || string(got) != "secret" {
		t.Fatalf("open: %q %v", got, err)
	}
	if _, err := box.Open(8, sealed); err == nil {
		t.Fatal("secret opened for another user")
	}
}
