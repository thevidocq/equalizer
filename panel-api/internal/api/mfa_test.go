package api_test

import (
	"encoding/base32"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/mfa"
)

func secretOf(t *testing.T, b32 string) []byte {
	t.Helper()
	s, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(b32)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestMFAFlow(t *testing.T) {
	e := setupWith(t, true)
	c := e.client()
	c.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)

	var me auth.User
	c.must(200, "GET", "/v1/auth/me", nil, &me)
	if me.MFAEnabled || me.MFAVerified {
		t.Fatalf("before enrolment: %+v", me)
	}
	// A password alone opens nothing but /auth.
	c.must(403, "GET", "/v1/tenants", nil, nil)
	c.must(409, "POST", "/v1/auth/mfa/verify", map[string]string{"code": "123456"}, nil)

	var setup struct {
		Secret     string `json:"secret"`
		OtpauthURI string `json:"otpauth_uri"`
	}
	c.must(200, "POST", "/v1/auth/mfa/setup", nil, &setup)
	if !strings.HasPrefix(setup.OtpauthURI, "otpauth://totp/Equalizer:root@example.com?") {
		t.Fatalf("uri: %s", setup.OtpauthURI)
	}
	secret := secretOf(t, setup.Secret)
	c.must(400, "POST", "/v1/auth/mfa/enable", map[string]string{"code": "000000"}, nil)
	var rec struct {
		RecoveryCodes []string `json:"recovery_codes"`
	}
	firstCode := mfa.Code(secret, mfa.Step(time.Now()))
	c.must(200, "POST", "/v1/auth/mfa/enable", map[string]string{"code": firstCode}, &rec)
	if len(rec.RecoveryCodes) != 10 {
		t.Fatalf("recovery codes: %v", rec.RecoveryCodes)
	}
	c.must(200, "GET", "/v1/tenants", nil, nil)

	// Next login: password, then the code; the same code is refused twice.
	c2 := e.client()
	c2.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	c2.must(403, "GET", "/v1/tenants", nil, nil)
	c2.must(409, "POST", "/v1/auth/mfa/setup", nil, nil) // cannot replace the app without the second step
	c2.must(400, "POST", "/v1/auth/mfa/verify", map[string]string{"code": firstCode}, nil)
	c2.must(200, "POST", "/v1/auth/mfa/verify", map[string]string{"code": rec.RecoveryCodes[0]}, nil)
	c2.must(200, "GET", "/v1/tenants", nil, nil)

	// A recovery code works once.
	c3 := e.client()
	c3.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	c3.must(400, "POST", "/v1/auth/mfa/verify", map[string]string{"code": rec.RecoveryCodes[0]}, nil)
	for i := 0; i < 4; i++ {
		c3.do("POST", "/v1/auth/mfa/verify", map[string]string{"code": "999999"}, nil)
	}
	if code := c3.do("POST", "/v1/auth/mfa/verify", map[string]string{"code": rec.RecoveryCodes[1]}, nil); code != http.StatusTooManyRequests {
		t.Fatalf("after 5 wrong codes: %d", code)
	}
}

func TestPasswordChangeAndUsers(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	boss := e.join("boss@acme.test")
	boss.must(201, "POST", "/v1/tenants/acme/invitations", map[string]string{"email": "ed@acme.test", "name": "Ed", "role": "editor"}, nil)
	a := e.join("ed@acme.test")
	var ed auth.User
	a.must(200, "GET", "/v1/auth/me", nil, &ed)
	b := e.client()
	b.must(200, "POST", "/v1/auth/login", map[string]string{"email": "ed@acme.test", "password": "a-long-password-1"}, nil)
	a.must(400, "POST", "/v1/auth/password", map[string]string{"current_password": "wrong", "new_password": "another-long-pass"}, nil)
	a.must(400, "POST", "/v1/auth/password", map[string]string{"current_password": "a-long-password-1", "new_password": "short"}, nil)
	a.must(204, "POST", "/v1/auth/password", map[string]string{"current_password": "a-long-password-1", "new_password": "another-long-pass"}, nil)
	a.must(200, "GET", "/v1/auth/me", nil, nil)
	b.must(401, "GET", "/v1/auth/me", nil, nil) // the other browser was logged out
	if e.client().login("ed@acme.test", "a-long-password-1") != http.StatusUnauthorized {
		t.Fatal("old password still works")
	}

	path := "/v1/tenants/acme/users/" + itoa(ed.ID)
	a.must(403, "DELETE", path, nil, nil)    // editors do not manage people
	root.must(403, "DELETE", path, nil, nil) // nor does the platform
	boss.must(204, "POST", path+"/reset-mfa", nil, nil)
	a.must(401, "GET", "/v1/auth/me", nil, nil) // reset ends the sessions
	boss.must(204, "DELETE", path, nil, nil)
	if e.client().login("ed@acme.test", "another-long-pass") != http.StatusUnauthorized {
		t.Fatal("deactivated user can log in")
	}
	boss.must(404, "DELETE", path, nil, nil)
}

func itoa(n int64) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{digits[n%10]}, b...)
	}
	return string(b)
}
