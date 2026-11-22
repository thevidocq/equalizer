// Package api serves the panel-api HTTP endpoints (schema/api/panel-api.yaml).
package api

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/exposure"
	"github.com/thevidocq/equalizer/panel-api/internal/health"
	"github.com/thevidocq/equalizer/panel-api/internal/llmmodels"
	"github.com/thevidocq/equalizer/panel-api/internal/logging"
	"github.com/thevidocq/equalizer/panel-api/internal/logsearch"
	"github.com/thevidocq/equalizer/panel-api/internal/mail"
	"github.com/thevidocq/equalizer/panel-api/internal/mapcheck"
	"github.com/thevidocq/equalizer/panel-api/internal/mfa"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// Server holds what the handlers need.
type Server struct {
	// CertRoots are the CAs a site certificate must chain to; nil: the
	// system's (a company's own CA can still come with the upload).
	CertRoots *x509.CertPool
	// TrustedProxies may set X-Forwarded-For (see clientIP).
	TrustedProxies []netip.Prefix
	Store          *store.Store
	Sessions       *auth.Sessions
	Throttle       *auth.Throttle
	Log            *slog.Logger
	SigningKey     ed25519.PrivateKey
	KeyID          string
	InternalToken  string
	UI             http.Handler // static panel-ui files; nil: API only
	MFA            *mfa.Box     // encrypts authenticator secrets
	MFARequired    bool         // every panel user must enrol an authenticator
	MFAThrottle    *auth.Throttle
	Mail           mail.Sender // invitations
	PanelURL       string      // base of the links in emails, e.g. https://panel.example.com
	RelayURL       string      // public address connectors dial, for their install command; empty: unknown
	// OutboundIPs: the addresses Equalizer connects from (EQ_OUTBOUND_IPS);
	// empty: those of the panel's and relay's domains.
	OutboundIPs []string
	Models      llmmodels.Lister // asks AI providers for their models; zero value: the public APIs
	// LookupHost resolves the platform's own domains (tests replace it).
	LookupHost    func(ctx context.Context, host string) ([]string, error)
	MailAvailable bool   // the platform sends email (alerts can work)
	Version       string // release of the platform; older connectors are outdated
	// Destinations a map may reach in direct mode (EQ_DIRECT_DESTINATIONS);
	// empty: tenant maps must use a connector.
	DirectDestinations map[string]bool
	Exposure           *exposure.Runner // nil: no exposure checks
	ExposureThrottle   *auth.Throttle
	Logs               *logsearch.Loki   // nil: the Logs page answers 503
	Health             *health.Probe     // nil: connectors and services shown as unknown
	MapCheck           *mapcheck.Checker // nil: maps are checked against the schema only
}

type userKey struct{}

// Handler returns the routed handler with logging and security middleware.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	// Auth
	mux.HandleFunc("POST /v1/auth/login", s.login)
	mux.HandleFunc("POST /v1/auth/logout", s.partial(s.logout))
	mux.HandleFunc("GET /v1/auth/me", s.partial(s.me))
	mux.HandleFunc("PATCH /v1/auth/me", s.authed(s.updateMe))
	mux.HandleFunc("POST /v1/auth/mfa/setup", s.partial(s.mfaSetup))
	mux.HandleFunc("POST /v1/auth/mfa/enable", s.partial(s.mfaEnable))
	mux.HandleFunc("POST /v1/auth/mfa/verify", s.partial(s.mfaVerify))
	mux.HandleFunc("POST /v1/auth/password", s.authed(s.changePassword))
	mux.HandleFunc("POST /v1/auth/password-reset", s.requestReset)
	mux.HandleFunc("GET /v1/auth/password-reset/{token}", s.viewReset)
	mux.HandleFunc("POST /v1/auth/password-reset/{token}", s.finishReset)
	// Tenants and users
	mux.HandleFunc("GET /v1/tenants", s.authed(s.listTenants))
	mux.HandleFunc("POST /v1/tenants", s.authed(s.createTenant))
	mux.HandleFunc("GET /v1/tenants/{tenant}/users", s.authed(s.listUsers))
	mux.HandleFunc("GET /v1/tenants/{tenant}/invitations", s.authed(s.listInvitations))
	mux.HandleFunc("POST /v1/tenants/{tenant}/invitations", s.authed(s.createInvitation))
	mux.HandleFunc("DELETE /v1/tenants/{tenant}/invitations/{id}", s.authed(s.revokeInvitation))
	mux.HandleFunc("GET /v1/invitations/{token}", s.viewInvitation)
	mux.HandleFunc("POST /v1/invitations/{token}/accept", s.acceptInvitation)
	mux.HandleFunc("GET /v1/platform/tenants", s.authed(s.platformTenants))
	mux.HandleFunc("POST /v1/platform/tenants/{tenant}/invitation", s.authed(s.reinviteFirstAdmin))
	mux.HandleFunc("DELETE /v1/platform/tenants/{tenant}", s.authed(s.deleteTenant))
	mux.HandleFunc("GET /v1/about", s.authed(s.about))
	mux.HandleFunc("DELETE /v1/tenants/{tenant}/users/{id}", s.authed(s.deactivateUser))
	mux.HandleFunc("POST /v1/tenants/{tenant}/users/{id}/reset-mfa", s.authed(s.resetMFA))
	// Apps and connectors
	mux.HandleFunc("GET /v1/tenants/{tenant}/apps", s.authed(s.listApps))
	mux.HandleFunc("POST /v1/tenants/{tenant}/apps", s.authed(s.createApp))
	mux.HandleFunc("GET /v1/apps/{app}", s.authed(s.getApp))
	mux.HandleFunc("PATCH /v1/apps/{app}", s.authed(s.patchApp))
	mux.HandleFunc("DELETE /v1/apps/{app}", s.authed(s.deleteApp))
	mux.HandleFunc("PUT /v1/apps/{app}/reach", s.authed(s.putReach))
	mux.HandleFunc("PUT /v1/apps/{app}/aliases", s.authed(s.putAliases))
	mux.HandleFunc("PUT /v1/apps/{app}/certificate", s.authed(s.putCertificate))
	mux.HandleFunc("DELETE /v1/apps/{app}/certificate", s.authed(s.deleteCertificate))
	mux.HandleFunc("POST /v1/apps/{app}/exposure/check", s.authed(s.checkExposure))
	mux.HandleFunc("GET /v1/tenants/{tenant}/connectors", s.authed(s.listConnectors))
	mux.HandleFunc("POST /v1/tenants/{tenant}/connectors", s.authed(s.createConnector))
	mux.HandleFunc("POST /v1/tenants/{tenant}/connectors/{id}/rotate", s.authed(s.rotateConnector))
	mux.HandleFunc("DELETE /v1/tenants/{tenant}/connectors/{id}", s.authed(s.deleteConnector))
	// Maps
	mux.HandleFunc("GET /v1/map-templates", s.authed(s.listTemplates))
	// Discovery and the tenant's AI
	mux.HandleFunc("GET /v1/tenants/{tenant}/alerts", s.authed(s.getAlerts))
	mux.HandleFunc("PUT /v1/tenants/{tenant}/alerts", s.authed(s.putAlerts))
	mux.HandleFunc("GET /v1/tenants/{tenant}/llm", s.authed(s.getLLM))
	mux.HandleFunc("PUT /v1/tenants/{tenant}/llm", s.authed(s.putLLM))
	mux.HandleFunc("DELETE /v1/tenants/{tenant}/llm", s.authed(s.deleteLLM))
	mux.HandleFunc("POST /v1/tenants/{tenant}/llm/models", s.authed(s.llmModels))
	mux.HandleFunc("GET /v1/apps/{app}/discoveries", s.authed(s.listDiscoveries))
	mux.HandleFunc("POST /v1/apps/{app}/discoveries", s.authed(s.startDiscovery))
	mux.HandleFunc("GET /v1/apps/{app}/discoveries/{id}", s.authed(s.getDiscovery))
	mux.HandleFunc("POST /v1/apps/{app}/discoveries/{id}/adopt", s.authed(s.adoptDiscovery))
	mux.HandleFunc("POST /v1/apps/{app}/map-template", s.authed(s.mapTemplate))
	mux.HandleFunc("GET /v1/apps/{app}/maps", s.authed(s.listMaps))
	mux.HandleFunc("POST /v1/apps/{app}/maps", s.authed(s.createMap))
	mux.HandleFunc("GET /v1/apps/{app}/maps/{version}", s.authed(s.getMap))
	mux.HandleFunc("GET /v1/apps/{app}/maps/{version}/review", s.authed(s.getReview))
	mux.HandleFunc("POST /v1/apps/{app}/maps/{version}/review", s.authed(s.askReview))
	mux.HandleFunc("POST /v1/apps/{app}/maps/{version}/{action}", s.authed(s.mapAction))
	mux.HandleFunc("GET /v1/apps/{app}/maps/{version}/verify", s.authed(s.getVerification))
	mux.HandleFunc("POST /v1/apps/{app}/maps/{version}/verify", s.authed(s.postVerification))
	// Audit
	mux.HandleFunc("GET /v1/tenants/{tenant}/logs", s.authed(s.listLogs))
	mux.HandleFunc("GET /v1/tenants/{tenant}/logs/stats", s.authed(s.logStats))
	mux.HandleFunc("GET /v1/tenants/{tenant}/logs/export", s.authed(s.exportLogs))
	mux.HandleFunc("GET /v1/apps/{app}/oidc", s.authed(s.getOIDC))
	mux.HandleFunc("PUT /v1/apps/{app}/oidc/secret", s.authed(s.putOIDCSecret))
	mux.HandleFunc("DELETE /v1/apps/{app}/oidc/secret", s.authed(s.deleteOIDCSecret))
	mux.HandleFunc("GET /v1/tenants/{tenant}/status", s.authed(s.tenantStatus))
	mux.HandleFunc("GET /v1/status/platform", s.authed(s.platformStatus))
	mux.HandleFunc("GET /v1/audit", s.authed(s.listAudit))
	// Internal services
	mux.HandleFunc("GET /v1/internal/certificates", s.internal(s.internalCertificates))
	mux.HandleFunc("GET /v1/internal/apps/{app}/map", s.internal(s.internalMap))
	mux.HandleFunc("GET /v1/internal/maps", s.internal(s.internalMaps))
	mux.HandleFunc("POST /v1/internal/discoveries/claim", s.internal(s.claimDiscovery))
	mux.HandleFunc("POST /v1/internal/verifications/claim", s.internal(s.claimVerification))
	mux.HandleFunc("POST /v1/internal/verifications/{id}/result", s.internal(s.verificationResult))
	mux.HandleFunc("POST /v1/internal/verifications/{id}/try-map", s.internal(s.verificationTryMap))
	mux.HandleFunc("POST /v1/internal/discoveries/{id}/progress", s.internal(s.discoveryProgress))
	mux.HandleFunc("POST /v1/internal/discoveries/{id}/result", s.internal(s.discoveryResult))
	mux.HandleFunc("POST /v1/internal/unmapped", s.internal(s.internalUnmapped))
	mux.HandleFunc("POST /v1/internal/reviews/claim", s.internal(s.claimReview))
	mux.HandleFunc("POST /v1/internal/reviews/{id}/result", s.internal(s.reviewResult))
	mux.HandleFunc("GET /v1/internal/connectors", s.internal(s.internalConnectors))
	mux.HandleFunc("GET /v1/internal/oidc", s.internal(s.internalOIDC))
	mux.HandleFunc("/v1/", func(w http.ResponseWriter, r *http.Request) { fail(w, http.StatusNotFound, "not_found", "") })
	if s.UI != nil {
		mux.Handle("/", s.UI)
	}
	return s.middleware(mux)
}

func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := newRequestID()
		ctx := logging.WithRequestID(r.Context(), id)
		r = r.WithContext(ctx)
		h := w.Header()
		h.Set(logging.HeaderRequestID, id)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		// Inline style attributes are allowed (dialog positioning); scripts only from this origin.
		h.Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; "+
			"object-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		if strings.HasPrefix(r.URL.Path, "/v1/") {
			h.Set("Cache-Control", "no-store")
		}

		// CSRF: state-changing requests must carry a header a cross-site form
		// cannot set (together with SameSite=Strict cookies).
		if unsafeMethod(r.Method) && r.Header.Get("X-Eq-Panel") != "1" && !isInternal(r) {
			fail(w, http.StatusForbidden, "csrf_header_missing", "send the header X-Eq-Panel: 1")
			s.logRequest(r, http.StatusForbidden, start)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
		rec := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.logRequest(r, rec.status, start)
	})
}

func (s *Server) logRequest(r *http.Request, status int, start time.Time) {
	s.Log.InfoContext(r.Context(), "request", slog.Group("detail",
		slog.String("method", r.Method), slog.String("path", r.URL.Path), slog.Int("status", status),
		slog.Float64("duration_ms", float64(time.Since(start).Microseconds())/1000)))
}

// authed requires a session that passed the second step.
func (s *Server) authed(h func(http.ResponseWriter, *http.Request, *auth.User)) http.HandlerFunc {
	return s.partial(func(w http.ResponseWriter, r *http.Request, u *auth.User) {
		if !u.MFAVerified {
			fail(w, http.StatusForbidden, "mfa_required", "complete the second login step")
			return
		}
		h(w, r, u)
	})
}

// partial requires a session, even one still waiting for the second step
// (only the /auth endpoints use it).
func (s *Server) partial(h func(http.ResponseWriter, *http.Request, *auth.User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(auth.CookieName)
		if err != nil {
			fail(w, http.StatusUnauthorized, "unauthorized", "")
			return
		}
		u, err := s.Sessions.User(r.Context(), c.Value)
		if err != nil {
			if err != auth.ErrNoSession {
				s.Log.ErrorContext(r.Context(), "session lookup failed", slog.String("error", err.Error()))
				fail(w, http.StatusInternalServerError, "internal", "")
				return
			}
			fail(w, http.StatusUnauthorized, "unauthorized", "")
			return
		}
		h(w, r.WithContext(context.WithValue(r.Context(), userKey{}, u)), u)
	}
}

func unsafeMethod(m string) bool {
	return m != http.MethodGet && m != http.MethodHead && m != http.MethodOptions
}

func isInternal(r *http.Request) bool {
	return len(r.URL.Path) >= len("/v1/internal/") && r.URL.Path[:len("/v1/internal/")] == "/v1/internal/"
}

func newRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "p-" + hex.EncodeToString(b)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
