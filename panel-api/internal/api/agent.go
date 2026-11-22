package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/thevidocq/equalizer/panel-api/internal/mapschema"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
	"github.com/thevidocq/equalizer/panel-api/internal/templates"
)

// The agent's side of discovery (service token): take a job, report
// progress, hand in the report and the proposed map.

type discoveryJob struct {
	ID         int64            `json:"id"`
	Tenant     string           `json:"tenant"`
	App        string           `json:"app"`
	Domain     string           `json:"domain"`
	Kind       *string          `json:"kind"`
	BaseURL    string           `json:"base_url"`
	Reach      json.RawMessage  `json:"reach"`
	CurrentMap json.RawMessage  `json:"current_map"`
	Template   json.RawMessage  `json:"template"`
	Account    map[string]any   `json:"account"`
	Accounts   []testAccount    `json:"accounts"`
	LLM        map[string]any   `json:"llm"`
	MaxPages   int              `json:"max_pages"`
	Unmapped   []store.Unmapped `json:"unmapped"`
}

func (s *Server) claimDiscovery(w http.ResponseWriter, r *http.Request) {
	c, err := s.Store.ClaimDiscovery(r.Context())
	if errors.Is(err, store.ErrNotFound) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	var rr runReach
	_ = json.Unmarshal(c.Reach, &rr)
	job := discoveryJob{ID: c.ID, Tenant: c.App.Tenant, App: c.App.ID, Domain: c.App.PublicDomain, Kind: c.App.Kind,
		BaseURL: rr.BaseURL, Reach: rr.Reach, MaxPages: c.MaxPages}
	if v, err := s.Store.Published(r.Context(), c.App.ID); err == nil {
		job.CurrentMap = v.Document
	} else if c.App.Kind != nil {
		var reach struct {
			ConnectorID string `json:"connector_id"`
			Address     string `json:"address"`
		}
		_ = json.Unmarshal(rr.Reach, &reach)
		job.Template, _ = templates.Render(*c.App.Kind, templates.Fill{BaseURL: rr.BaseURL, Connector: reach.ConnectorID, Address: reach.Address})
		// The template is written for a connector; the reach chosen for this
		// site (direct or connector) replaces it, as for a template opened by hand.
		if job.Template != nil {
			if t, err := withReach(job.Template, rr.Reach); err == nil {
				job.Template = t
			}
		}
	}
	job.Accounts = s.jobAccounts(c)
	if len(job.Accounts) > 0 {
		// agents before the study read only the first account
		job.Account = map[string]any{"username": job.Accounts[0].Username, "password": job.Accounts[0].Password}
	}
	if c.UseAI {
		job.LLM = s.tenantModel(r, c.App.Tenant)
	}
	job.Unmapped, _ = s.Store.UnmappedOf(r.Context(), c.App.ID, 200)
	writeJSON(w, http.StatusOK, job)
}

// jobAccounts opens the test accounts of a claimed discovery: the sealed
// list, or the single account of a discovery started before it.
func (s *Server) jobAccounts(c store.Claim) []testAccount {
	label := "discovery:" + c.App.ID
	if c.AccountsSecret != nil {
		var out []testAccount
		if blob, err := s.MFA.OpenFor(label, c.AccountsSecret); err == nil && json.Unmarshal(blob, &out) == nil {
			return out
		}
		return nil
	}
	if c.AccountUser != nil && c.AccountSecret != nil {
		if pw, err := s.MFA.OpenFor(label, c.AccountSecret); err == nil {
			return []testAccount{{Label: "account", Username: *c.AccountUser, Password: string(pw)}}
		}
	}
	return nil
}

// tenantModel is the tenant's AI model with its key opened, for the agent;
// nil when the tenant has none.
func (s *Server) tenantModel(r *http.Request, tenant string) map[string]any {
	l, err := s.Store.TenantLLM(r.Context(), tenant)
	if err != nil {
		return nil
	}
	out := map[string]any{"provider": l.Provider, "model": l.Model, "endpoint": l.Endpoint}
	if len(l.Key) > 0 {
		if key, err := s.MFA.OpenFor(llmLabel(tenant), l.Key); err == nil {
			out["api_key"] = string(key)
		}
	}
	return out
}

func (s *Server) discoveryProgress(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, http.StatusNotFound, "not_found", "")
		return
	}
	var p json.RawMessage
	if !decode(w, r, &p) {
		return
	}
	if err := s.Store.DiscoveryProgress(r.Context(), id, p); err != nil {
		s.storeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// discoveryResult saves the report and, when the proposed map differs from
// the published one, a draft by whoever started the discovery. The draft
// has the same checks as any other and needs someone else's approval.
func (s *Server) discoveryResult(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, http.StatusNotFound, "not_found", "")
		return
	}
	var in struct {
		Report   json.RawMessage `json:"report"`
		Document json.RawMessage `json:"document"`
		Error    string          `json:"error"`
	}
	if !decode(w, r, &in) {
		return
	}
	appID, requester, err := s.Store.RunningDiscovery(r.Context(), id)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	a, err := s.Store.App(r.Context(), appID)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	var draft *int
	failure := ""
	if in.Error != "" {
		failure = truncate(in.Error, 500)
	} else if len(in.Document) > 0 && string(in.Document) != "null" {
		draft, failure = s.proposeDraft(r.Context(), a, id, requester, in.Document)
	}
	var fp *string
	if failure != "" {
		fp = &failure
	}
	// the draft arrives measured and corrected: the agent verifies it next,
	// behind the test accounts' logins too (docs/agent.md); the accounts
	// pass to the verification before the discovery erases its own
	opts := store.VerifyOptions{Correct: true, Author: requester}
	if c, err := s.Store.DiscoverySecrets(r.Context(), id); draft != nil && err == nil {
		opts.AccountsSecret, opts.Accounts = s.sealAccounts(a.ID, s.jobAccounts(c))
	}
	// queued before the discovery shows its draft: a person adding to the
	// draft (adopt) always sees the agent's check in progress
	if draft != nil {
		_, _ = s.Store.QueueVerification(r.Context(), a.ID, *draft, nil, opts)
	}
	if err := s.Store.FinishDiscovery(r.Context(), id, in.Report, draft, fp); err != nil {
		if draft != nil {
			// the discovery ended otherwise meanwhile: its draft is nobody's,
			// so no correction is made on it
			_ = s.Store.RetireDraft(r.Context(), a.ID, *draft)
		}
		s.storeError(w, r, err)
		return
	}
	detail := map[string]any{"id": id, "draft_version": draft}
	if fp != nil {
		detail["error"] = failure
	}
	_ = s.Store.Record(r.Context(), nil, &a.Tenant, "discovery.finish", a.ID, detail)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) proposeDraft(ctx context.Context, a store.App, id int64, requester *int64, doc json.RawMessage) (*int, string) {
	doc, failure := s.checkProposal(ctx, a, doc, fmt.Sprintf("agent:discovery-%d", id))
	if failure != "" {
		return nil, failure
	}
	if v, err := s.Store.Published(ctx, a.ID); err == nil && sameMap(v.Document, doc) {
		return nil, "" // nothing changed: no draft
	}
	if requester == nil {
		return nil, "nobody to author the draft (the person who started it was removed)"
	}
	version, err := s.Store.CreateDraft(ctx, a.ID, doc, fmt.Sprintf("Proposed by discovery #%d", id), *requester)
	if err != nil {
		return nil, "the draft could not be saved"
	}
	return &version, ""
}

// checkProposal gives a map the agent proposes the panel's identity fields
// and every check a draft gets; a message says why it is refused.
func (s *Server) checkProposal(ctx context.Context, a store.App, doc json.RawMessage, createdBy string) (json.RawMessage, string) {
	doc, err := store.SetMeta(doc, map[string]any{
		"schema": "gateway-map/1.0", "tenant": a.Tenant, "app": a.ID, "version": 1, "status": "draft",
		"created_by": createdBy, "public_domain": a.PublicDomain,
	})
	if err != nil {
		return nil, "the proposed map is not a JSON object"
	}
	if err := mapschema.Validate(doc); err != nil {
		var ie *mapschema.ErrInvalid
		if errors.As(err, &ie) && len(ie.Details) > 0 {
			return nil, truncate("the proposed map does not respect the schema: "+strings.Join(ie.Details, "; "), 500)
		}
		return nil, "the proposed map does not respect the schema"
	}
	if err := s.checkReach(ctx, a.Tenant, doc); err != nil {
		return nil, "the proposed map: " + err.Error()
	}
	if err := s.mapChecked(ctx, doc, false); err != nil {
		return nil, truncate("the proposed map: "+err.Error(), 500)
	}
	return doc, ""
}

// sameMap compares two maps without their meta.
func sameMap(a, b json.RawMessage) bool {
	norm := func(raw json.RawMessage) []byte {
		var m map[string]any
		if json.Unmarshal(raw, &m) != nil {
			return nil
		}
		delete(m, "meta")
		out, _ := json.Marshal(m)
		return out
	}
	x, y := norm(a), norm(b)
	return x != nil && bytes.Equal(x, y)
}

func (s *Server) internalUnmapped(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Items []store.Unmapped `json:"items"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.Items) > 1000 {
		fail(w, http.StatusUnprocessableEntity, "invalid", "at most 1000 items")
		return
	}
	clean := in.Items[:0]
	for _, u := range in.Items {
		if u.App == "" || u.Hits < 1 || u.Clients < 1 || !strings.HasPrefix(u.Path, "/") || len(u.Path) > 512 || len(u.Method) > 10 {
			continue
		}
		u.Visitors = visitorKeys(u.Visitors)
		clean = append(clean, u)
	}
	if err := s.Store.AddUnmapped(r.Context(), clean); err != nil {
		s.storeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// visitorKeys keeps the well-formed visitor keys a gateway sent (8 bytes in
// hex, at most 16): anything else is not a key and is dropped.
func visitorKeys(in []string) []string {
	out := make([]string, 0, len(in))
	for _, k := range in {
		if len(out) < 16 && visitorKey.MatchString(k) {
			out = append(out, k)
		}
	}
	return out
}

var visitorKey = regexp.MustCompile(`^[0-9a-f]{16}$`)

// ScheduleDiscoveries queues the continuous discoveries that are due. The
// site's published map says how to reach it; the owner authors the draft.
func (s *Server) ScheduleDiscoveries(ctx context.Context) (int, error) {
	due, err := s.Store.DueDiscoveries(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, d := range due {
		v, err := s.Store.Published(ctx, d.App)
		if err != nil {
			continue
		}
		a, err := s.Store.App(ctx, d.App)
		if err != nil {
			continue
		}
		var doc struct {
			Legacy struct {
				BaseURL string          `json:"base_url"`
				Reach   json.RawMessage `json:"reach"`
			} `json:"legacy"`
		}
		if json.Unmarshal(v.Document, &doc) != nil {
			continue
		}
		reach, _ := json.Marshal(runReach{BaseURL: doc.Legacy.BaseURL, Reach: doc.Legacy.Reach})
		_, useAI := s.Store.TenantLLM(ctx, a.Tenant)
		owner := d.Owner
		if _, err := s.Store.StartDiscovery(ctx, store.NewDiscovery{App: d.App, Trigger: "schedule", UseAI: useAI == nil,
			MaxPages: 500, Reach: reach, RequestedBy: &owner}); err == nil {
			n++
		}
	}
	return n, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
