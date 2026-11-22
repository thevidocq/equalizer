package api

import (
	"encoding/json"
	"errors"
	"fmt"
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

// postVerification queues a verification of a version (the Verify button),
// with test accounts for the pages behind their login when given.
func (s *Server) postVerification(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	if !u.HasTenantRole(a.Tenant, "admin") {
		forbidden(w)
		return
	}
	version, ok := pathInt(r, "version")
	if !ok {
		fail(w, http.StatusNotFound, "not_found", "")
		return
	}
	var in struct {
		Accounts []testAccount `json:"accounts"`
		Correct  bool          `json:"correct"`
	}
	if r.ContentLength != 0 && !decode(w, r, &in) {
		return
	}
	accounts, msg := testAccounts(discoveryStart{Accounts: in.Accounts})
	if msg != "" {
		fail(w, http.StatusUnprocessableEntity, "invalid", msg)
		return
	}
	if _, err := s.Store.Version(r.Context(), a.ID, version); err != nil {
		s.storeError(w, r, err)
		return
	}
	opts := store.VerifyOptions{Correct: in.Correct, Author: &u.ID}
	opts.AccountsSecret, opts.Accounts = s.sealAccounts(a.ID, accounts)
	v, err := s.Store.QueueVerification(r.Context(), a.ID, version, &u.ID, opts)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	_ = s.Store.Record(r.Context(), &u.ID, &a.Tenant, "map.verify", a.ID, map[string]any{"version": version, "accounts": len(accounts)})
	writeJSON(w, http.StatusAccepted, v)
}

// sealAccounts seals test accounts for a verification; nil when there are none.
func (s *Server) sealAccounts(app string, accounts []testAccount) ([]byte, []string) {
	if len(accounts) == 0 {
		return nil, nil
	}
	labels := make([]string, len(accounts))
	for i, a := range accounts {
		labels[i] = a.Label
	}
	blob, _ := json.Marshal(accounts)
	return s.MFA.SealFor("verification:"+app, blob), labels
}

type verificationJob struct {
	ID             int64               `json:"id"`
	Tenant         string              `json:"tenant"`
	App            string              `json:"app"`
	Kind           *string             `json:"kind"`
	Version        int                 `json:"version"`
	Document       json.RawMessage     `json:"document"`
	BaseURL        string              `json:"base_url"`
	Reach          json.RawMessage     `json:"reach"`
	Pages          []string            `json:"pages"`
	PagesByAccount map[string][]string `json:"pages_by_account"`
	TryMap         signing.TryMap      `json:"try_map"`
	Accounts       []testAccount       `json:"accounts"`
	Correct        bool                `json:"correct"`
}

// claimVerification hands the agent the next verification: the version,
// signed for tries on the gateway, the pages to compare and the accounts.
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
		_ = s.Store.FinishVerification(r.Context(), c.ID, store.VerificationResult{Failure: &msg})
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
	pages, byAccount, _ := s.Store.StudiedPages(r.Context(), a.ID)
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
	accounts := []testAccount{}
	if c.AccountsSecret != nil {
		if blob, err := s.MFA.OpenFor("verification:"+a.ID, c.AccountsSecret); err == nil {
			_ = json.Unmarshal(blob, &accounts)
		}
	}
	if byAccount == nil {
		byAccount = map[string][]string{}
	}
	writeJSON(w, http.StatusOK, verificationJob{ID: c.ID, Tenant: a.Tenant, App: a.ID, Kind: a.Kind, Version: c.Version,
		Document: mv.Document, BaseURL: doc.Legacy.BaseURL, Reach: doc.Legacy.Reach, Pages: pages,
		PagesByAccount: byAccount, TryMap: tm, Accounts: accounts, Correct: c.Correct && c.AuthorID != nil})
}

// verificationTryMap signs a version the agent corrected, for its next
// tries: the same checks as any draft of this site.
func (s *Server) verificationTryMap(w http.ResponseWriter, r *http.Request) {
	v, a, ok := s.runningVerification(w, r)
	if !ok {
		return
	}
	var in struct {
		Document json.RawMessage `json:"document"`
	}
	if !decode(w, r, &in) {
		return
	}
	doc, failure := s.checkProposal(r.Context(), a, in.Document, fmt.Sprintf("agent:verification-%d", v.ID))
	if failure != "" {
		fail(w, http.StatusUnprocessableEntity, "invalid_map", failure)
		return
	}
	tm, err := signing.SignTry(doc, time.Now().Add(tryLife), s.SigningKey, s.KeyID)
	if err != nil {
		fail(w, http.StatusUnprocessableEntity, "invalid_map", "the map could not be signed")
		return
	}
	writeJSON(w, http.StatusOK, tm)
}

func (s *Server) runningVerification(w http.ResponseWriter, r *http.Request) (store.Verification, store.App, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, http.StatusNotFound, "not_found", "")
		return store.Verification{}, store.App{}, false
	}
	v, err := s.Store.RunningVerification(r.Context(), id)
	if err != nil {
		s.storeError(w, r, err)
		return v, store.App{}, false
	}
	a, err := s.Store.App(r.Context(), v.App)
	if err != nil {
		s.storeError(w, r, err)
		return v, a, false
	}
	return v, a, true
}

// verificationResult stores the agent's score and findings; a corrected
// version becomes a new draft, carrying the agent's last measure of it.
func (s *Server) verificationResult(w http.ResponseWriter, r *http.Request) {
	v, a, ok := s.runningVerification(w, r)
	if !ok {
		return
	}
	var in struct {
		Score       json.RawMessage `json:"score"`
		Broken      json.RawMessage `json:"broken"`
		Open        json.RawMessage `json:"open"`
		Error       string          `json:"error"`
		Document    json.RawMessage `json:"document"`
		Corrections json.RawMessage `json:"corrections"`
		Before      json.RawMessage `json:"before"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !validList(in.Broken, 500) || !validList(in.Open, 1000) || !validList(in.Corrections, 500) {
		fail(w, http.StatusUnprocessableEntity, "invalid", "broken: at most 500 items, open: 1000, corrections: 500")
		return
	}
	res := store.VerificationResult{Score: in.Score, Broken: in.Broken, Open: in.Open}
	if in.Error != "" {
		msg := truncate(in.Error, 500)
		res.Failure = &msg
	}
	corrected := len(in.Document) > 0 && string(in.Document) != "null"
	if res.Failure == nil && corrected && v.Correct && v.AuthorID != nil {
		doc, failure := s.checkProposal(r.Context(), a, in.Document, fmt.Sprintf("agent:verification-%d", v.ID))
		if failure != "" {
			res.Failure = &failure
		} else if nv, err := s.Store.CreateDraft(r.Context(), a.ID, doc,
			fmt.Sprintf("Corrected by the agent from version %d", v.Version), *v.AuthorID); err == nil {
			// the corrected version carries its measure and the corrections that made it
			final := res
			final.Corrections, final.Before = in.Corrections, in.Before
			_ = s.Store.RecordVerification(r.Context(), a.ID, nv, v.Accounts, final)
			// this version keeps its own measure, before the corrections
			res = store.VerificationResult{Score: in.Before, Corrections: in.Corrections, Before: in.Before, CorrectedVersion: &nv}
			_ = s.Store.Record(r.Context(), nil, &a.Tenant, "map.corrected", a.ID, map[string]any{"from": v.Version, "version": nv})
			if v.RequestedBy == nil {
				// after a discovery: one draft to review, the corrected one
				if err := s.Store.Supersede(r.Context(), a.ID, v.Version, nv); err == nil {
					_ = s.Store.Record(r.Context(), nil, &a.Tenant, "map.superseded", a.ID, map[string]any{"from": v.Version, "version": nv})
				}
			}
		}
	}
	if err := s.Store.FinishVerification(r.Context(), v.ID, res); err != nil {
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
