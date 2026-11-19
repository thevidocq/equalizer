package api_test

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// resetToken waits for the reset email sent in the background to email.
func (e *env) resetToken(email string) string {
	e.t.Helper()
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		msgs := e.mail.Messages()
		for i := len(msgs) - 1; i >= 0; i-- {
			if j := strings.Index(msgs[i].Body, "/reset/"); msgs[i].To == email && j >= 0 {
				return strings.Fields(msgs[i].Body[j+len("/reset/"):])[0]
			}
		}
	}
	e.t.Fatalf("no reset email for %s", email)
	return ""
}

func TestPasswordReset(t *testing.T) {
	e := setup(t)
	old := e.client()
	old.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)

	anon := e.client()
	// Same answer for an unknown account, and no email.
	anon.must(http.StatusAccepted, "POST", "/v1/auth/password-reset", map[string]string{"email": "nobody@example.com"}, nil)
	anon.must(http.StatusAccepted, "POST", "/v1/auth/password-reset", map[string]string{"email": "Root@Example.com"}, nil)
	token := e.resetToken("root@example.com")
	for _, m := range e.mail.Messages() {
		if m.To == "nobody@example.com" {
			t.Fatal("email sent to an unknown account")
		}
	}

	anon.must(http.StatusNoContent, "GET", "/v1/auth/password-reset/"+token, nil, nil)
	anon.must(http.StatusNotFound, "GET", "/v1/auth/password-reset/forged", nil, nil)
	anon.must(http.StatusBadRequest, "POST", "/v1/auth/password-reset/"+token, map[string]string{"password": "short"}, nil)
	anon.must(http.StatusNoContent, "POST", "/v1/auth/password-reset/"+token, map[string]string{"password": "a-brand-new-password-1"}, nil)
	// Once only; the old sessions ended; the new password works, the old does not.
	anon.must(http.StatusNotFound, "POST", "/v1/auth/password-reset/"+token, map[string]string{"password": "another-password-12"}, nil)
	if code := old.do("GET", "/v1/auth/me", nil, nil); code != http.StatusUnauthorized {
		t.Errorf("old session still works: %d", code)
	}
	if e.client().login("root@example.com", "platform-admin-password") != http.StatusUnauthorized {
		t.Error("old password still works")
	}
	if e.client().login("root@example.com", "a-brand-new-password-1") != http.StatusOK {
		t.Error("new password refused")
	}

	// A few requests per window, then 429.
	flood := e.client()
	code := 0
	for i := 0; i < 10 && code != http.StatusTooManyRequests; i++ {
		code = flood.do("POST", "/v1/auth/password-reset", map[string]string{"email": "x@example.com"}, nil)
	}
	if code != http.StatusTooManyRequests {
		t.Error("reset requests not limited")
	}
}

func TestAlertSettings(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	admin := e.join("boss@acme.test")
	var got struct {
		Enabled     bool     `json:"enabled"`
		ExtraEmails []string `json:"extra_emails"`
	}
	admin.must(200, "GET", "/v1/tenants/acme/alerts", nil, &got)
	if !got.Enabled || len(got.ExtraEmails) != 0 {
		t.Fatalf("defaults: %+v", got)
	}
	admin.must(422, "PUT", "/v1/tenants/acme/alerts", map[string]any{"enabled": true, "extra_emails": []string{"not-an-email"}}, nil)
	admin.must(200, "PUT", "/v1/tenants/acme/alerts", map[string]any{"enabled": false, "extra_emails": []string{" OnCall@Acme.test "}}, &got)
	admin.must(200, "GET", "/v1/tenants/acme/alerts", nil, &got)
	if got.Enabled || len(got.ExtraEmails) != 1 || got.ExtraEmails[0] != "oncall@acme.test" {
		t.Errorf("saved: %+v", got)
	}
	root.must(403, "GET", "/v1/tenants/acme/alerts", nil, nil) // the platform sees no tenant settings
}
