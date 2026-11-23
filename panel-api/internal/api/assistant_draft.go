package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/thevidocq/equalizer/panel-api/internal/assistant"
	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/mapcheck"
	"github.com/thevidocq/equalizer/panel-api/internal/mapschema"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

var errChecking = errors.New("the latest draft is still being checked: try again once its check is done")

// proposalBase is the version a proposal goes on: the newest version not
// yet published, when it is a draft not being checked; else the published
// map. A version under review stops it: a draft built beside it would undo
// it once published. fromDraft says which.
func (s *Server) proposalBase(ctx context.Context, app string) (store.MapVersion, bool, error) {
	vs, err := s.Store.Versions(ctx, app)
	if err != nil {
		return store.MapVersion{}, false, errors.New("the map versions could not be read")
	}
	for _, v := range vs { // newest first
		switch v.Status {
		case "retired", "rejected":
			continue
		case "in_review", "approved":
			return store.MapVersion{}, false, fmt.Errorf("version %d is under review: publish or reject it first", v.Version)
		case "draft":
			if vf, err := s.Store.LatestVerification(ctx, app, v.Version); err == nil && (vf.Status == "queued" || vf.Status == "running") {
				return store.MapVersion{}, false, errChecking
			}
			full, err := s.Store.Version(ctx, app, v.Version)
			if err != nil {
				return store.MapVersion{}, false, errors.New("the map could not be read")
			}
			return full, true, nil
		}
		break // published: the base below
	}
	v, err := s.Store.Published(ctx, app)
	if err != nil {
		return v, false, errors.New("the site has no published map nor draft yet: run a discovery first")
	}
	return v, false, nil
}

// draftDoc is doc as a draft of the site, checked against the map schema.
func (s *Server) draftDoc(a store.App, u *auth.User, doc map[string]any) (json.RawMessage, error) {
	raw, _ := json.Marshal(doc)
	out, err := store.SetMeta(raw, map[string]any{
		"schema": "gateway-map/1.0", "tenant": a.Tenant, "app": a.ID, "version": 1, "status": "draft",
		"created_by": "user:" + u.Email, "public_domain": a.PublicDomain,
	})
	if err != nil {
		return nil, err
	}
	if err := mapschema.Validate(out); err != nil {
		var ie *mapschema.ErrInvalid
		if errors.As(err, &ie) {
			return nil, fmt.Errorf("%s", strings.Join(ie.Details, "; "))
		}
		return nil, err
	}
	return out, nil
}

type draftFail struct {
	status     int
	code, text string
}

func (e *draftFail) Error() string { return e.text }

// draftProposal turns a proposal into a new draft (on the latest draft or
// the published map) and has the agent check it, as any draft. One at a
// time per site: two clicks never build two drafts on the same base.
func (s *Server) draftProposal(w http.ResponseWriter, r *http.Request, u *auth.User) {
	a, ok := s.app(w, r, u)
	if !ok {
		return
	}
	if !u.HasTenantRole(a.Tenant, "admin") {
		forbidden(w)
		return
	}
	var out map[string]any
	err := s.Store.WithAppLock(r.Context(), a.ID, func() error {
		var err error
		out, err = s.makeDraft(r, a, u, r.PathValue("proposal"))
		return err
	})
	var df *draftFail
	switch {
	case errors.As(err, &df):
		fail(w, df.status, df.code, df.text)
	case err != nil:
		s.storeError(w, r, err)
	default:
		writeJSON(w, http.StatusCreated, out)
	}
}

func (s *Server) makeDraft(r *http.Request, a store.App, u *auth.User, id string) (map[string]any, error) {
	ctx := r.Context()
	p, err := s.Store.Proposal(ctx, a.ID, id)
	if err != nil {
		return nil, err
	}
	if p.DraftVersion != nil {
		return nil, &draftFail{http.StatusConflict, "already_drafted", fmt.Sprintf("already in draft version %d", *p.DraftVersion)}
	}
	var c assistant.Change
	if err := json.Unmarshal(p.Change, &c); err != nil {
		return nil, err
	}
	base, fromDraft, err := s.proposalBase(ctx, a.ID)
	if err != nil {
		return nil, &draftFail{http.StatusConflict, "no_base", err.Error()}
	}
	var doc map[string]any
	if err := json.Unmarshal(base.Document, &doc); err != nil {
		return nil, err
	}
	summary, err := c.Apply(doc) // checked again: the map may have changed since
	if errors.Is(err, assistant.ErrSame) {
		return nil, &draftFail{http.StatusConflict, "already_there", "version " + strconv.Itoa(base.Version) + " already has it"}
	}
	if err != nil {
		return nil, &draftFail{http.StatusConflict, "not_applicable", err.Error()}
	}
	out, err := s.draftDoc(a, u, doc)
	if err != nil {
		return nil, &draftFail{http.StatusUnprocessableEntity, "invalid_map", "the map would not be valid: " + err.Error()}
	}
	if err := s.mapChecked(ctx, out, false); err != nil {
		var inc *mapcheck.Inconsistent
		switch {
		case errors.As(err, &inc):
			return nil, &draftFail{http.StatusUnprocessableEntity, "map_inconsistent", "The gateway would refuse this map: " + inc.Reason}
		case errors.Is(err, mapcheck.ErrUnavailable):
			return nil, &draftFail{http.StatusServiceUnavailable, "map_unchecked", "No gateway could check the map now: try again in a moment."}
		}
		return nil, err
	}
	note := fmt.Sprintf("From version %d, proposed by the assistant: %s", base.Version, summary)
	version, err := s.Store.CreateDraft(ctx, a.ID, out, note, u.ID)
	if err != nil {
		return nil, err
	}
	undo := func(e error) (map[string]any, error) {
		_ = s.Store.RetireDraft(ctx, a.ID, version)
		return nil, e
	}
	if err := s.Store.ProposalDrafted(ctx, a.ID, p.ID, version); err != nil {
		return undo(&draftFail{http.StatusConflict, "already_drafted", "this proposal was just turned into a draft"})
	}
	// one draft to review: the new one takes the place of the one it was built on
	if fromDraft {
		if err := s.Store.Supersede(ctx, a.ID, base.Version, version); err != nil {
			return undo(&draftFail{http.StatusConflict, "draft_taken", fmt.Sprintf("version %d was taken on meanwhile: try again", base.Version)})
		}
	}
	_, _ = s.Store.QueueVerification(ctx, a.ID, version, &u.ID, store.VerifyOptions{Author: &u.ID})
	_ = s.Store.Record(ctx, &u.ID, &a.Tenant, "assistant.draft", a.ID,
		map[string]any{"version": version, "from": base.Version, "kind": c.Kind})
	return map[string]any{"app": a.ID, "version": version, "status": "draft"}, nil
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
