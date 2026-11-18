package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/mail"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// InvitationTTL is how long an invitation link works.
const InvitationTTL = 48 * time.Hour

var tenantRoles = map[string]bool{"admin": true, "editor": true, "approver": true, "auditor": true}

var roleNames = map[string]string{"admin": "administrator", "editor": "editor", "approver": "approver", "auditor": "auditor"}

func tokenHash(t string) string {
	sum := sha256.Sum256([]byte(t))
	return hex.EncodeToString(sum[:])
}

// invite stores an invitation and emails its link. The link is never
// returned by the API nor written to the logs (except EQ_MAIL_MODE=log).
func (s *Server) invite(r *http.Request, by *auth.User, inv store.Invitation) (store.Invitation, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return inv, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	inv.ExpiresAt = time.Now().Add(InvitationTTL)
	saved, err := s.Store.CreateInvitation(r.Context(), inv, tokenHash(token), by.ID)
	if err != nil {
		return saved, err
	}
	saved.TenantName = inv.TenantName
	msg := mail.Message{
		To:      saved.Email,
		Subject: fmt.Sprintf("You are invited to the Equalizer panel of %s", inv.TenantName),
		Body: fmt.Sprintf("Hello %s,\n\nyou are invited to join %s on the Equalizer panel as %s.\n\n"+
			"Open this link within 48 hours to choose your password:\n%s/invite/%s\n\n"+
			"Then link an authenticator app on your phone: the panel asks for its code at every login.\n\n"+
			"If you did not expect this email, ignore it.\n", saved.Name, inv.TenantName, roleNames[saved.Role],
			strings.TrimSuffix(s.PanelURL, "/"), token),
	}
	if err := s.Mail.Send(r.Context(), msg); err != nil {
		s.Log.ErrorContext(r.Context(), "invitation email not sent", slog.String("error", err.Error()),
			slog.Group("detail", slog.Int64("invitation_id", saved.ID)))
		return saved, nil
	}
	saved.Sent = true
	return saved, s.Store.MarkSent(r.Context(), saved.ID)
}

func (s *Server) listInvitations(w http.ResponseWriter, r *http.Request, u *auth.User) {
	tenant := r.PathValue("tenant")
	if !u.HasTenantRole(tenant, "admin") {
		forbidden(w)
		return
	}
	invs, err := s.Store.Invitations(r.Context(), tenant)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list(invs))
}

type invitationCreate struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

func (s *Server) createInvitation(w http.ResponseWriter, r *http.Request, u *auth.User) {
	tenant := r.PathValue("tenant")
	if !u.HasTenantRole(tenant, "admin") {
		forbidden(w)
		return
	}
	var in invitationCreate
	if !decode(w, r, &in) {
		return
	}
	in.Email, in.Name = strings.TrimSpace(in.Email), strings.TrimSpace(in.Name)
	if !validEmail(in.Email) || in.Name == "" || len(in.Name) > 200 || !tenantRoles[in.Role] {
		fail(w, http.StatusUnprocessableEntity, "invalid", "email, name and role (admin, editor, approver, auditor) are required")
		return
	}
	ts, err := s.Store.Tenants(r.Context(), &tenant)
	if err != nil || len(ts) == 0 {
		s.storeError(w, r, store.ErrNotFound)
		return
	}
	inv, err := s.invite(r, u, store.Invitation{Tenant: tenant, TenantName: ts[0].Name, Email: in.Email, Name: in.Name, Role: in.Role})
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &tenant, "user.invite", inv.Email, map[string]any{"role": inv.Role, "sent": inv.Sent})
	writeJSON(w, http.StatusCreated, inv)
}

func (s *Server) revokeInvitation(w http.ResponseWriter, r *http.Request, u *auth.User) {
	tenant := r.PathValue("tenant")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || !u.HasTenantRole(tenant, "admin") {
		forbidden(w)
		return
	}
	if err := s.Store.RevokeInvitation(r.Context(), tenant, id); err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &tenant, "user.invite_revoked", strconv.FormatInt(id, 10), nil)
	w.WriteHeader(http.StatusNoContent)
}

// viewInvitation is public: the token in the link is the secret.
func (s *Server) viewInvitation(w http.ResponseWriter, r *http.Request) {
	inv, err := s.Store.InvitationByToken(r.Context(), tokenHash(r.PathValue("token")))
	if err != nil {
		fail(w, http.StatusNotFound, "invitation_not_valid", "the link expired, was already used or was revoked")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tenant_name": inv.TenantName, "email": inv.Email, "name": inv.Name,
		"role": inv.Role, "expires_at": inv.ExpiresAt})
}

func (s *Server) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		fail(w, http.StatusUnprocessableEntity, "weak_password", err.Error())
		return
	}
	u, err := s.Store.AcceptInvitation(r.Context(), tokenHash(r.PathValue("token")), hash)
	if err != nil {
		if err == store.ErrConflict {
			fail(w, http.StatusConflict, "already_exists", "an account with this email already exists")
			return
		}
		fail(w, http.StatusNotFound, "invitation_not_valid", "the link expired, was already used or was revoked")
		return
	}
	u.MFAVerified = !s.MFARequired
	id, err := s.Sessions.Create(r.Context(), u.ID, u.MFAVerified)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.setSessionCookie(w, id)
	_ = s.Store.Record(r.Context(), &u.ID, u.Tenant, "user.join", u.Email, map[string]any{"role": *u.TenantRole})
	writeJSON(w, http.StatusCreated, u)
}
