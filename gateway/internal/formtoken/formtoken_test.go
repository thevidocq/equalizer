package formtoken

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSealOpen(t *testing.T) {
	s, err := New(RandomKey())
	if err != nil {
		t.Fatal(err)
	}
	in := State{Form: "contact", Params: map[string]string{"id": "42"}, Hidden: map[string]string{"viewstate": "secret-internal"}, Charset: "iso-8859-1"}
	tok, err := s.Seal(in, "cookie-a")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(tok, "secret") || strings.Contains(tok, "contact") {
		t.Fatal("the token must not reveal its content")
	}
	out, err := s.Open(tok, "cookie-a")
	if err != nil {
		t.Fatal(err)
	}
	if out.Form != "contact" || out.Params["id"] != "42" || out.Hidden["viewstate"] != "secret-internal" || out.Charset != "iso-8859-1" {
		t.Fatalf("state not preserved: %+v", out)
	}

	if _, err := s.Open(tok, "cookie-b"); !errors.Is(err, ErrBinding) {
		t.Errorf("other browser: %v", err)
	}
	if _, err := s.Open(tok, ""); !errors.Is(err, ErrBinding) {
		t.Errorf("no cookie: %v", err)
	}
	tampered := []byte(tok)
	tampered[len(tampered)/2] ^= 1
	if _, err := s.Open(string(tampered), "cookie-a"); !errors.Is(err, ErrInvalid) {
		t.Errorf("tampered token: %v", err)
	}
	for _, bad := range []string{"", "!!", "AAAA"} {
		if _, err := s.Open(bad, "cookie-a"); !errors.Is(err, ErrInvalid) {
			t.Errorf("%q: %v", bad, err)
		}
	}

	other, _ := New(RandomKey())
	if _, err := other.Open(tok, "cookie-a"); !errors.Is(err, ErrInvalid) {
		t.Errorf("other key: %v", err)
	}

	s.now = func() time.Time { return time.Now().Add(3 * time.Hour) }
	if _, err := s.Open(tok, "cookie-a"); !errors.Is(err, ErrExpired) {
		t.Errorf("expired: %v", err)
	}
}

func TestNewRejectsShortKey(t *testing.T) {
	if _, err := New([]byte("short")); err == nil {
		t.Fatal("a short key must be rejected")
	}
}
