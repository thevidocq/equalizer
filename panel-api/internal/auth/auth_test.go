package auth

import (
	"strings"
	"testing"
	"time"
)

func TestPassword(t *testing.T) {
	h, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "pbkdf2-sha256$600000$") || strings.Contains(h, "horse") {
		t.Fatalf("hash format: %s", h)
	}
	if !CheckPassword(h, "correct horse battery") || CheckPassword(h, "correct horse batterY") {
		t.Fatal("check failed")
	}
	for _, bad := range []string{"", "x$y", "pbkdf2-sha256$0$aa$bb", "md5$1$a$b"} {
		if CheckPassword(bad, "x") {
			t.Errorf("%q accepted", bad)
		}
	}
	if _, err := HashPassword("short"); err == nil {
		t.Error("short password accepted")
	}
}

func TestRoles(t *testing.T) {
	admin, op, ed, tn := "admin", "operator", "editor", "acme"
	pa := &User{PlatformRole: &admin}
	po := &User{PlatformRole: &op}
	te := &User{Tenant: &tn, TenantRole: &ed}
	// Platform users run the service but see nothing inside tenants.
	if pa.HasTenantRole("acme", "approver") || pa.CanRead("acme") || po.HasTenantRole("acme", "editor") || po.CanRead("acme") {
		t.Error("platform roles must not reach tenant data")
	}
	if !te.HasTenantRole("acme", "editor", "admin") || te.HasTenantRole("acme", "approver") || te.CanRead("other") {
		t.Error("tenant roles")
	}
}

func TestThrottle(t *testing.T) {
	th := NewThrottle(3, time.Minute)
	for i := 0; i < 3; i++ {
		if th.Blocked("ip:1") {
			t.Fatalf("blocked after %d", i)
		}
		th.Fail("ip:1", "email:a")
	}
	if !th.Blocked("ip:1") || !th.Blocked("ip:9", "email:a") || th.Blocked("ip:2") {
		t.Fatal("throttle state")
	}
	th.Reset("ip:1", "email:a")
	if th.Blocked("ip:1") {
		t.Fatal("reset")
	}
}
