package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/signing"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// tryLife is how long a version signed for the agent's tries stays valid
// on the gateway (at most two hours there).
const tryLife = time.Hour

// maxVerifyPages bounds the pages one verification compares.
const maxVerifyPages = 300

// getVerification answers the latest verification of a version.
func (s *Server) getVerification(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	version, ok := pathInt(r, "version")
	if !ok {
		fail(w, http.StatusNotFound, "not_found", "")
		return
	}
	v, err := s.Store.LatestVerification(r.Context(), a.ID, version)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// postVerification queues a verification of a version (the Verify button).
func (s *Server) postVerification(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	if !u.HasTenantRole(a.Tenant, "admin", "editor", "approver") {
		forbidden(w)
		return
	}
	version, ok := pathInt(r, "version")
	if !ok {
		fail(w, http.StatusNotFound, "not_found", "")
		return
	}
	if _, err := s.Store.Version(r.Context(), a.ID, version); err != nil {
		s.storeError(w, r, err)
		return
	}
	v, err := s.Store.QueueVerification(r.Context(), a.ID, version, &u.ID)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &a.Tenant, "map.verify", a.ID, map[string]any{"version": version})
	writeJSON(w, http.StatusAccepted, v)
}

type verificationJob struct {
	ID       int64               `json:"id"`
	Tenant   string              `json:"tenant"`
	App      string              `json:"app"`
	Version  int                 `json:"version"`
	Document json.RawMessage     `json:"document"`
	BaseURL  string              `json:"base_url"`
	Reach    json.RawMessage     `json:"reach"`
	Pages    []string            `json:"pages"`
	TryMap   signing.TryMap      `json:"try_map"`
	Accounts []map[string]string `json:"accounts"`
}

// claimVerification hands the agent the next verification: the version,
// signed for tries on the gateway, and the pages to compare.
func (s *Server) claimVerification(w http.ResponseWriter, r *http.Request) {
	c, err := s.Store.ClaimVerification(r.Context())
	if errors.Is(err, store.ErrNotFound) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	a, err := s.Store.App(r.Context(), c.App)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	mv, err := s.Store.Version(r.Context(), c.App, c.Version)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	failure := func(msg string) {
		_ = s.Store.FinishVerification(r.Context(), c.ID, nil, nil, nil, &msg)
		w.WriteHeader(http.StatusNoContent)
	}
	var doc struct {
		Legacy struct {
			BaseURL string          `json:"base_url"`
			Reach   json.RawMessage `json:"reach"`
		} `json:"legacy"`
		Routes []struct {
			Path   string `json:"path"`
			Method string `json:"method"`
		} `json:"routes"`
	}
	if json.Unmarshal(mv.Document, &doc) != nil || doc.Legacy.BaseURL == "" {
		failure("the version has no legacy site")
		return
	}
	tm, err := signing.SignTry(mv.Document, time.Now().Add(tryLife), s.SigningKey, s.KeyID)
	if err != nil {
		failure("the version could not be signed for tries")
		return
	}
	pages, _ := s.Store.StudiedPages(r.Context(), a.ID)
	if len(pages) == 0 { // no discovery yet: the version's own literal pages
		for _, rt := range doc.Routes {
			if (rt.Method == "" || rt.Method == http.MethodGet) && !strings.Contains(rt.Path, "{") {
				pages = append(pages, rt.Path)
			}
		}
	}
	if len(pages) > maxVerifyPages {
		pages = pages[:maxVerifyPages]
	}
	writeJSON(w, http.StatusOK, verificationJob{ID: c.ID, Tenant: a.Tenant, App: a.ID, Version: c.Version,
		Document: mv.Document, BaseURL: doc.Legacy.BaseURL, Reach: doc.Legacy.Reach, Pages: pages, TryMap: tm,
		Accounts: []map[string]string{}})
}

// verificationResult stores the agent's score and findings.
func (s *Server) verificationResult(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, http.StatusNotFound, "not_found", "")
		return
	}
	var in struct {
		Score  json.RawMessage `json:"score"`
		Broken json.RawMessage `json:"broken"`
		Open   json.RawMessage `json:"open"`
		Error  string          `json:"error"`
	}
	if !decode(w, r, &in) {
		return
	}
	var failure *string
	if in.Error != "" {
		msg := truncate(in.Error, 500)
		failure = &msg
	}
	if !validList(in.Broken, 500) || !validList(in.Open, 1000) {
		fail(w, http.StatusUnprocessableEntity, "invalid", "broken: at most 500 items, open: at most 1000")
		return
	}
	if err := s.Store.FinishVerification(r.Context(), id, in.Score, in.Broken, in.Open, failure); err != nil {
		s.storeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validList(raw json.RawMessage, max int) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return true
	}
	var items []json.RawMessage
	return json.Unmarshal(raw, &items) == nil && len(items) <= max
}
