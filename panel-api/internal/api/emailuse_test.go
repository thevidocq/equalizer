package api_test

import "testing"

// An invitation to an address that already has an account, or a pending
// invitation, is refused when sent, not when accepted.
func TestInvitationEmailInUse(t *testing.T) {
	e := setup(t)
	root := e.client()
	root.must(200, "POST", "/v1/auth/login", map[string]string{"email": "root@example.com", "password": "platform-admin-password"}, nil)
	root.must(409, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "ROOT@example.com", "admin_name": "Root"}, nil)
	root.must(201, "POST", "/v1/tenants", map[string]string{"id": "acme", "name": "Acme", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)
	root.must(409, "POST", "/v1/tenants", map[string]string{"id": "other", "name": "Other", "admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)

	var ov struct {
		Items []struct {
			ID           string
			PendingAdmin *struct{ Name, Email string } `json:"pending_admin"`
		}
	}
	root.must(200, "GET", "/v1/platform/tenants", nil, &ov)
	if len(ov.Items) != 1 || ov.Items[0].PendingAdmin == nil || ov.Items[0].PendingAdmin.Email != "boss@acme.test" {
		t.Fatalf("overview: %+v", ov)
	}
	// Inviting the same person again replaces their own pending invitation.
	root.must(201, "POST", "/v1/platform/tenants/acme/invitation", map[string]string{"admin_email": "boss@acme.test", "admin_name": "Boss"}, nil)

	boss := e.join("boss@acme.test")
	ov.Items = nil
	root.must(200, "GET", "/v1/platform/tenants", nil, &ov)
	if ov.Items[0].PendingAdmin != nil {
		t.Fatalf("joined tenant still shows a pending administrator: %+v", ov.Items[0].PendingAdmin)
	}
	boss.must(409, "POST", "/v1/tenants/acme/invitations", map[string]string{"email": "root@example.com", "name": "Root", "role": "admin"}, nil)
	boss.must(201, "POST", "/v1/tenants/acme/invitations", map[string]string{"email": "ed@acme.test", "name": "Ed", "role": "admin"}, nil)
	boss.must(409, "POST", "/v1/tenants/acme/invitations", map[string]string{"email": "ed@acme.test", "name": "Ed", "role": "admin"}, nil)
}
