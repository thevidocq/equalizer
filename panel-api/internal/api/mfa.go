package api

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/mfa"
)

const issuer = "Equalizer"

type mfaCode struct {
	Code string `json:"code"`
}

// mfaSetup starts an enrolment. A user who already has an authenticator
// must pass the second step before replacing it.
func (s *Server) mfaSetup(w http.ResponseWriter, r *http.Request, u *auth.User) {
	if u.MFAEnabled && !u.MFAVerified {
		fail(w, http.StatusConflict, "mfa_already_enabled", "use the code from your app")
		return
	}
	secret := mfa.NewSecret()
	if err := s.Store.SetMFAPending(r.Context(), u.ID, s.MFA.Seal(u.ID, secret)); err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"secret": mfa.Base32(secret), "otpauth_uri": mfa.URI(secret, issuer, u.Email)})
}

// mfaEnable confirms the enrolment with a first code and completes the login.
func (s *Server) mfaEnable(w http.ResponseWriter, r *http.Request, u *auth.User) {
	if u.MFAEnabled && !u.MFAVerified {
		fail(w, http.StatusConflict, "mfa_already_enabled", "use the code from your app")
		return
	}
	var in mfaCode
	if !decode(w, r, &in) || !s.mfaAllowed(w, u) {
		return
	}
	st, err := s.Store.MFA(r.Context(), u.ID)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	if st.Pending == nil {
		fail(w, http.StatusConflict, "mfa_not_started", "start the setup first")
		return
	}
	secret, err := s.MFA.Open(u.ID, st.Pending)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	step, ok := mfa.Verify(secret, in.Code, time.Now(), 0)
	if !ok {
		s.mfaFailed(w, r, u)
		return
	}
	codes, hashes := mfa.RecoveryCodes(10)
	if err := s.Store.EnableMFA(r.Context(), u.ID, st.Pending, step, hashes); err != nil {
		s.storeError(w, r, err)
		return
	}
	if !s.upgrade(w, r, u) {
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, u.Tenant, "auth.mfa_enabled", u.Email, nil)
	writeJSON(w, http.StatusOK, map[string]any{"recovery_codes": codes})
}

// mfaVerify is the second login step: a code from the app or a recovery code.
func (s *Server) mfaVerify(w http.ResponseWriter, r *http.Request, u *auth.User) {
	if !u.MFAEnabled {
		fail(w, http.StatusConflict, "mfa_not_enabled", "set up an authenticator app first")
		return
	}
	var in mfaCode
	if !decode(w, r, &in) || !s.mfaAllowed(w, u) {
		return
	}
	st, err := s.Store.MFA(r.Context(), u.ID)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	secret, err := s.MFA.Open(u.ID, st.Secret)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	method := "app"
	ok := false
	if step, valid := mfa.Verify(secret, in.Code, time.Now(), st.LastStep); valid {
		ok, err = s.Store.UseStep(r.Context(), u.ID, step)
	} else if len(in.Code) > 6 {
		method = "recovery_code"
		ok, err = s.Store.UseRecoveryCode(r.Context(), u.ID, mfa.HashRecovery(in.Code))
	}
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	if !ok {
		s.mfaFailed(w, r, u)
		return
	}
	if !s.upgrade(w, r, u) {
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, u.Tenant, "auth.mfa_verified", "", map[string]any{"method": method})
	u.MFAVerified = true
	writeJSON(w, http.StatusOK, u)
}

// upgrade gives the session a new id marked as complete.
func (s *Server) upgrade(w http.ResponseWriter, r *http.Request, u *auth.User) bool {
	c, err := r.Cookie(auth.CookieName)
	if err != nil {
		fail(w, http.StatusUnauthorized, "unauthorized", "")
		return false
	}
	id, err := s.Sessions.Upgrade(r.Context(), c.Value, u.ID)
	if err != nil {
		s.storeError(w, r, err)
		return false
	}
	s.MFAThrottle.Reset(throttleKey(u))
	s.setSessionCookie(w, id)
	return true
}

func throttleKey(u *auth.User) string { return "user:" + strconv.FormatInt(u.ID, 10) }

func (s *Server) mfaAllowed(w http.ResponseWriter, u *auth.User) bool {
	if s.MFAThrottle.Blocked(throttleKey(u)) {
		fail(w, http.StatusTooManyRequests, "too_many_attempts", "try again later")
		return false
	}
	return true
}

func (s *Server) mfaFailed(w http.ResponseWriter, r *http.Request, u *auth.User) {
	s.MFAThrottle.Fail(throttleKey(u))
	s.Log.WarnContext(r.Context(), "second login step failed", slog.Group("detail", slog.Int64("user_id", u.ID)))
	_ = s.Store.Record(r.Context(), &u.ID, u.Tenant, "auth.mfa_failed", "", map[string]any{"ip": clientIP(r)})
	fail(w, http.StatusBadRequest, "invalid_code", "the code is wrong or expired")
}
