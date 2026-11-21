package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/review"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// The approver's aid: fixed rules always, the tenant's AI when configured.
// It informs; approving stays a person's decision.

type mapReview struct {
	Version     int              `json:"version"`
	BaseVersion *int             `json:"base_version"`
	Changes     []review.Change  `json:"changes"`
	Findings    []review.Finding `json:"findings"`
	AI          store.AIReview   `json:"ai"`
}

func (s *Server) versionFor(w http.ResponseWriter, r *http.Request, u *auth.User) (store.App, store.MapVersion, bool) {
	a, ok := s.app(w, r, u)
	if !ok {
		return a, store.MapVersion{}, false
	}
	version, ok := pathInt(r, "version")
	if !ok {
		fail(w, http.StatusNotFound, "not_found", "")
		return a, store.MapVersion{}, false
	}
	v, err := s.Store.Version(r.Context(), a.ID, version)
	if err != nil {
		s.storeError(w, r, err)
		return a, v, false
	}
	return a, v, true
}

// compare returns the version this one is compared with: the published one
// for a version not yet live, the one it replaced for a version that went
// live (published or retired since).
func (s *Server) compare(r *http.Request, a store.App, v store.MapVersion) mapReview {
	out := mapReview{Version: v.Version}
	var base json.RawMessage
	p, err := s.Store.Published(r.Context(), a.ID)
	if v.PublishedAt != nil {
		p, err = s.Store.PublishedBefore(r.Context(), a.ID, v.Version)
	}
	if err == nil && p.Version != v.Version {
		base, out.BaseVersion = p.Document, &p.Version
	}
	res := review.Compare(base, v.Document)
	out.Changes, out.Findings = res.Changes, res.Findings
	return out
}

func (s *Server) getReview(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, v, ok := s.versionFor(w, r, u)
	if !ok {
		return
	}
	out := s.compare(r, a, v)
	ai, err := s.Store.LatestReview(r.Context(), a.ID, v.Version)
	switch {
	case err == nil:
		out.AI = ai
	case errors.Is(err, store.ErrNotFound) && (v.Status == "draft" || v.Status == "in_review") && s.hasLLM(r, a.Tenant):
		// First look at a version not yet published: ask the AI now.
		_ = s.Store.QueueReview(r.Context(), a.ID, v.Version)
		out.AI = store.AIReview{Status: "queued", Questions: []string{}}
	default:
		out.AI = store.AIReview{Status: "off", Questions: []string{}}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) askReview(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, v, ok := s.versionFor(w, r, u)
	if !ok {
		return
	}
	if !u.HasTenantRole(a.Tenant, "admin") {
		forbidden(w)
		return
	}
	if !s.hasLLM(r, a.Tenant) {
		fail(w, http.StatusConflict, "no_ai", "no AI model is configured for this organisation")
		return
	}
	if err := s.Store.QueueReview(r.Context(), a.ID, v.Version); errors.Is(err, store.ErrConflict) {
		fail(w, http.StatusConflict, "review_running", "an AI review of this version is already queued or running")
		return
	} else if err != nil {
		s.storeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// claimReview gives the agent the comparison and the tenant's model. The
// AI sees the changes and findings, never the map's values or site content.
func (s *Server) claimReview(w http.ResponseWriter, r *http.Request) {
	c, err := s.Store.ClaimReview(r.Context())
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
	v, err := s.Store.Version(r.Context(), c.App, c.Version)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	job := map[string]any{"id": c.ID, "app": a.ID, "kind": a.Kind, "version": v.Version, "note": v.Note, "review": s.compare(r, a, v)}
	if llm := s.tenantModel(r, a.Tenant); llm != nil {
		job["llm"] = llm
	} else {
		failure := "no AI model is configured for this organisation"
		_ = s.Store.FinishReview(r.Context(), c.ID, "", nil, &failure)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) reviewResult(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, http.StatusNotFound, "not_found", "")
		return
	}
	var in struct {
		Summary   string   `json:"summary"`
		Questions []string `json:"questions"`
		Error     string   `json:"error"`
	}
	if !decode(w, r, &in) {
		return
	}
	var failure *string
	if in.Error != "" {
		e := truncate(in.Error, 500)
		failure = &e
	}
	qs := []string{}
	for i, q := range in.Questions {
		if i == 20 {
			break
		}
		qs = append(qs, truncate(q, 400))
	}
	if err := s.Store.FinishReview(r.Context(), id, truncate(in.Summary, 3000), qs, failure); err != nil {
		s.storeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
