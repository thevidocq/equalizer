package assistant

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"regexp"
	"strings"
)

// Masker keeps personal data from the AI provider. A visitor's address
// becomes a code (V-3fa9c1), the same for the same address in a
// conversation and never turned back; a panel user's email becomes U-…,
// which the answer shown to the person turns back; an email inside a path
// becomes user@…
type Masker struct {
	key   []byte
	users map[string]string // code → email
}

// NewMasker uses key, or a random one.
func NewMasker(key []byte) *Masker {
	if len(key) == 0 {
		key = make([]byte, 32)
		_, _ = rand.Read(key)
	}
	return &Masker{key: key, users: map[string]string{}}
}

func (m *Masker) code(prefix, v string) string {
	h := hmac.New(sha256.New, m.key)
	h.Write([]byte(prefix + "\x00" + v))
	return prefix + "-" + hex.EncodeToString(h.Sum(nil)[:3])
}

// Visitor is the code of a client address.
func (m *Masker) Visitor(ip string) string {
	if ip == "" {
		return ""
	}
	return m.code("V", ip)
}

// User is the code of a panel user, turned back by Unmask.
func (m *Masker) User(email string) string {
	if email == "" {
		return ""
	}
	c := m.code("U", strings.ToLower(email))
	m.users[c] = email
	return c
}

var (
	emailRe = regexp.MustCompile(`[^\s/?&=@"'<>]+@[^\s/?&=@"'<>]+\.[A-Za-z]{2,}`)
	ipRe    = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b|[0-9A-Fa-f:]*::[0-9A-Fa-f:.]*|\b(?:[0-9A-Fa-f]{1,4}:){7}[0-9A-Fa-f]{1,4}\b`)
)

// Text hides the emails in a path, a query or a free text, and turns IP
// addresses into visitor codes.
func (m *Masker) Text(s string) string {
	s = emailRe.ReplaceAllString(s, "user@…")
	return ipRe.ReplaceAllStringFunc(s, func(ip string) string {
		if net.ParseIP(ip) == nil {
			return ip
		}
		return m.Visitor(ip)
	})
}

// Unmask turns the user codes in an answer back into emails: the person
// asking sees names, the provider never did.
func (m *Masker) Unmask(s string) string {
	for c, e := range m.users {
		s = strings.ReplaceAll(s, c, e)
	}
	return s
}

// fence wraps what came from the site, its visitors or its logs: a path an
// attacker chose can say "ignore your instructions", and the model must read
// it as data. The marker is random, so the data cannot close it.
func fence(content string) string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	tag := "data-" + hex.EncodeToString(b)
	return "<" + tag + ">\n" + strings.ReplaceAll(content, tag, "data") + "\n</" + tag + ">"
}
