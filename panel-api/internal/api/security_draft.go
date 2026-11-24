package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/auth"
	"github.com/thevidocq/equalizer/panel-api/internal/mapcheck"
	"github.com/thevidocq/equalizer/panel-api/internal/securityscan"
	"github.com/thevidocq/equalizer/panel-api/internal/store"
)

// After a scan, the panel builds one map draft that hardens the inputs the
// published map serves as free text, on the latest draft or the published
// map, as the assistant's drafts: one draft to review, in the Map tab,
// checked by the agent like any other.

// draftOutcome is what building the draft of a scan came to.
type draftOutcome struct {
	version *int
	note    string
	fixes   map[string]string
	busy    bool // the map could not take a draft now (a version in review, a draft being checked): try again
}

// HardenAfterScan builds the draft of an ended scan, once (claimed, so two
// replicas or a retry never build two), and records it on the scan: its
// version and what it hardens, or why there is none. When the map is busy
// it leaves the scan to the runner's retry, for a day.
func (s *Server) HardenAfterScan(ctx context.Context, id int64) {
	if ok, err := s.Store.ClaimSecurityDraft(ctx, id); err != nil || !ok {
		return
	}
	sc, err := s.Store.SecurityScanByID(ctx, id)
	if err != nil {
		return
	}
	out := s.hardenScan(ctx, sc)
	if out.busy && sc.CompletedAt != nil && time.Since(*sc.CompletedAt) < 24*time.Hour {
		return // claimed for 5 minutes, then tried again
	}
	if err := s.Store.SetSecurityDraft(ctx, sc.ID, out.version, out.note, out.fixes); err != nil {
		s.Log.Error("security draft not recorded", slog.Int64("scan", sc.ID), slog.String("error", err.Error()))
	}
}

func (s *Server) hardenScan(ctx context.Context, sc store.SecurityScan) draftOutcome {
	findings, err := s.Store.SecurityFindings(ctx, sc.ID)
	if err != nil {
		return draftOutcome{note: "no draft: the findings could not be read"}
	}
	pub, err := s.Store.Published(ctx, sc.App)
	if err != nil {
		return draftOutcome{note: "no draft: the site has no published map"}
	}
	published, err := securityscan.ParseMap(pub.Document)
	if err != nil {
		return draftOutcome{note: "no draft: the published map could not be read"}
	}
	a, err := s.Store.App(ctx, sc.App)
	if err != nil {
		return draftOutcome{note: "no draft: the site could not be read"}
	}
	var out draftOutcome
	err = s.Store.WithAppLock(ctx, a.ID, func() error {
		out = s.buildSecurityDraft(ctx, a, sc, published, findings)
		return nil
	})
	if err != nil {
		return draftOutcome{busy: true, note: "no draft: the site's map could not be locked"}
	}
	return out
}

func (s *Server) buildSecurityDraft(ctx context.Context, a store.App, sc store.SecurityScan, published *securityscan.Map,
	findings []store.SecurityFinding) draftOutcome {
	// A version waiting in the Map tab may harden them already (an earlier
	// scan's draft): then it is the one to point at, and no new draft.
	if v, doc, ok := s.waitingVersion(ctx, a.ID); ok {
		h := securityscan.Harden(doc, published, findings)
		if len(h.Added) == 0 {
			if len(h.Already) == 0 {
				return draftOutcome{note: nothingToHarden}
			}
			return draftOutcome{version: &v, fixes: fixMap(h.Already),
				note: fmt.Sprintf("Version %d, waiting in the Map tab, already %s", v, hardens(h.Already))}
		}
	}
	base, fromDraft, err := s.proposalBase(ctx, a.ID)
	if err != nil {
		return draftOutcome{busy: true, note: "no draft: " + err.Error()}
	}
	var doc map[string]any
	if err := json.Unmarshal(base.Document, &doc); err != nil {
		return draftOutcome{note: "no draft: the map could not be read"}
	}
	h := securityscan.Harden(doc, published, findings)
	if len(h.Added) == 0 {
		if len(h.Already) > 0 && !fromDraft {
			return draftOutcome{note: fmt.Sprintf("the published map already has the text rules for %d finding(s), "+
				"and the scanner still found them: they need a fix on the site or a narrower type", len(h.Already))}
		}
		return draftOutcome{note: nothingToHarden}
	}
	author := s.scanAuthor(ctx, sc)
	if author == nil {
		return draftOutcome{note: "no draft: the person who started the scan is no longer an administrator of the site"}
	}
	raw, err := s.draftDoc(a, author, doc)
	if err != nil {
		return draftOutcome{note: "no draft: the map would not be valid: " + err.Error()}
	}
	if err := s.mapChecked(ctx, raw, false); err != nil {
		var inc *mapcheck.Inconsistent
		if errors.As(err, &inc) {
			return draftOutcome{note: "no draft: the gateway would refuse it: " + inc.Reason}
		}
		return draftOutcome{busy: true, note: "no draft: no gateway could check the map"}
	}
	all := append(append([]securityscan.Fix{}, h.Added...), h.Already...)
	what := hardens(all)
	note := fmt.Sprintf("From version %d, after the security scan of %s (%s): %s", base.Version,
		sc.Domain, sc.CreatedAt.UTC().Format("2 Jan 2006"), what)
	version, err := s.Store.CreateDraft(ctx, a.ID, raw, note, author.ID)
	if err != nil {
		return draftOutcome{busy: true, note: "no draft: it could not be saved"}
	}
	if fromDraft {
		if err := s.Store.Supersede(ctx, a.ID, base.Version, version); err != nil {
			_ = s.Store.RetireDraft(ctx, a.ID, version)
			return draftOutcome{busy: true, note: fmt.Sprintf("no draft: version %d was taken on meanwhile", base.Version)}
		}
	}
	_, _ = s.Store.QueueVerification(ctx, a.ID, version, &author.ID, store.VerifyOptions{Author: &author.ID})
	tenant := a.Tenant
	_ = s.Store.Record(ctx, nil, &tenant, "security.draft", a.ID,
		map[string]any{"version": version, "from": base.Version, "hardens": len(all), "target": sc.Target})
	return draftOutcome{version: &version, fixes: fixMap(all), note: fmt.Sprintf("Draft version %d %s", version, what)}
}

const nothingToHarden = "nothing the map can harden by itself: the open findings need a precise entry (a discovery) or a fix on the server"

// waitingVersion is the newest version not yet published (a draft, or one
// in review), with its document.
func (s *Server) waitingVersion(ctx context.Context, app string) (int, map[string]any, bool) {
	vs, err := s.Store.Versions(ctx, app)
	if err != nil {
		return 0, nil, false
	}
	for _, v := range vs { // newest first
		switch v.Status {
		case "retired", "rejected":
			continue
		case "draft", "in_review", "approved":
			full, err := s.Store.Version(ctx, app, v.Version)
			var doc map[string]any
			if err != nil || json.Unmarshal(full.Document, &doc) != nil {
				return 0, nil, false
			}
			return v.Version, doc, true
		}
		return 0, nil, false // published is the newest
	}
	return 0, nil, false
}

// scanAuthor is the person who started the scan, while still an
// administrator of the site's tenant: the draft is theirs.
func (s *Server) scanAuthor(ctx context.Context, sc store.SecurityScan) *auth.User {
	if sc.RequestedByID == nil {
		return nil
	}
	users, err := s.Store.Users(ctx, sc.Tenant)
	if err != nil {
		return nil
	}
	for i := range users {
		if users[i].ID == *sc.RequestedByID && users[i].TenantRole != nil && *users[i].TenantRole == "admin" {
			return &users[i]
		}
	}
	return nil
}

func hardens(fixes []securityscan.Fix) string {
	changes := make([]string, len(fixes))
	for i, f := range fixes {
		changes[i] = f.Change
	}
	what := fmt.Sprintf("hardens %d finding(s): %s", len(fixes), strings.Join(dedupe(changes), "; "))
	if r := []rune(what); len(r) > 900 {
		what = string(r[:900]) + "…"
	}
	return what
}

func dedupe(list []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range list {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func fixMap(fixes []securityscan.Fix) map[string]string {
	out := map[string]string{}
	for _, f := range fixes {
		out[f.Finding] = f.Change
	}
	return out
}
