package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/mail"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// ResetTTL is how long a forgotten password link works.
const ResetTTL = time.Hour

// requestReset emails a reset link when the account exists. The answer and
// its timing are the same either way: the email is sent in the background.
func (s *Server) requestReset(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &in) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	keys := []string{"reset-ip:" + s.clientIP(r), "reset-email:" + email}
	if s.Throttle.Blocked(keys...) {
		fail(w, http.StatusTooManyRequests, "too_many_attempts", "try again later")
		return
	}
	s.Throttle.Fail(keys...) // every request counts: a few per window
	w.WriteHeader(http.StatusAccepted)
	if !validEmail(email) {
		return
	}
	go s.sendReset(context.WithoutCancel(r.Context()), email, s.clientIP(r))
}

func (s *Server) sendReset(ctx context.Context, email, ip string) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	u, _, err := s.Store.Credentials(ctx, email)
	if err != nil {
		return // no such active account: nothing is sent, nothing is said
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	if err := s.Store.CreateReset(ctx, u.ID, tokenHash(token), time.Now().Add(ResetTTL)); err != nil {
		s.Log.ErrorContext(ctx, "password reset not stored", slog.String("error", err.Error()))
		return
	}
	_ = s.Store.Record(ctx, &u.ID, u.Tenant, "auth.password_reset_requested", u.Email, map[string]any{"ip": ip})
	msg := mail.Message{
		To:      u.Email,
		Subject: "Choose a new password for the Equalizer panel",
		Body: fmt.Sprintf("Hello %s,\n\nsomeone asked to choose a new password for your account on the Equalizer panel.\n\n"+
			"Open this link within one hour:\n%s/reset/%s\n\n"+
			"Your authenticator app stays as it is: the panel still asks for its code.\n\n"+
			"If it was not you, ignore this email: your password does not change.\n",
			u.Name, strings.TrimSuffix(s.PanelURL, "/"), token),
	}
	if err := s.Mail.Send(ctx, msg); err != nil {
		s.Log.ErrorContext(ctx, "password reset email not sent", slog.String("error", err.Error()))
	}
}

func (s *Server) viewReset(w http.ResponseWriter, r *http.Request) {
	ok, err := s.Store.ResetValid(r.Context(), tokenHash(r.PathValue("token")))
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	if !ok {
		fail(w, http.StatusNotFound, "not_found", "the link has expired or was already used")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) finishReset(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		fail(w, http.StatusBadRequest, "weak_password", err.Error())
		return
	}
	id, tenant, err := s.Store.ConsumeReset(r.Context(), tokenHash(r.PathValue("token")))
	if errors.Is(err, store.ErrNotFound) {
		fail(w, http.StatusNotFound, "not_found", "the link has expired or was already used")
		return
	}
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	if err := s.Store.SetPassword(r.Context(), id, hash); err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Sessions.EndAll(r.Context(), id, "") // every browser logs in again
	_ = s.Store.Record(r.Context(), &id, tenant, "auth.password_reset", "", map[string]any{"ip": s.clientIP(r)})
	w.WriteHeader(http.StatusNoContent)
}
